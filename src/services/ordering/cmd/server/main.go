package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/messaging"
	bbmiddleware "github.com/osmanozen/oo-commerce/pkg/buildingblocks/middleware"
	orderinghttp "github.com/osmanozen/oo-commerce/services/ordering/internal/adapters/http"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/adapters/persistence"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/application/commands"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/application/queries"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/saga"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("ordering service starting",
		slog.String("version", "1.0.0"),
	)

	port := envOrDefault("PORT", "8083")
	kafkaBrokers := []string{envOrDefault("KAFKA_BROKERS", "localhost:9092")}
	databaseURL := envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ocommerce?sslmode=disable")

	// ─── Kafka Producer ─────────────────────────────────────────────────
	kafkaCfg := messaging.DefaultKafkaProducerConfig(kafkaBrokers)
	kafkaProducer := messaging.NewKafkaProducer(kafkaCfg, logger)
	defer kafkaProducer.Close()

	// ─── Kafka Consumer ─────────────────────────────────────────────────
	kafkaConsumer := messaging.NewKafkaConsumer(kafkaBrokers, logger)
	defer kafkaConsumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ─── Database + Repositories + Saga ─────────────────────────────────
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		logger.Error("failed to parse database url", slog.String("error", err.Error()))
		os.Exit(1)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Error("failed to initialize database pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("database ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	orderRepo := persistence.NewOrderRepository(pool, logger)
	cartReader := persistence.NewCartReader(pool)
	sagaStore := persistence.NewSagaStore(pool)
	checkoutSaga := saga.NewCheckoutSaga(kafkaProducer, sagaStore, logger)
	sagaConsumer := saga.NewSagaEventConsumer(checkoutSaga, kafkaConsumer, logger)
	if err := sagaConsumer.Start(ctx); err != nil {
		logger.Error("failed to start saga consumers", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// ─── HTTP Router ────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(bbmiddleware.CorrelationID)
	r.Use(bbmiddleware.RequestLogger(logger))
	r.Use(bbmiddleware.Recovery(logger))
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Compress(5))
	r.Use(chimiddleware.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy","service":"ordering"}`)
	})

	checkoutHandler := commands.NewCheckoutHandler(cartReader, orderRepo, checkoutSaga)
	cancelOrderHandler := commands.NewCancelOrderHandler(orderRepo)
	simulatePaymentHandler := commands.NewSimulatePaymentHandler(kafkaProducer, sagaStore)
	getOrderByIDHandler := queries.NewGetOrderByIDHandler(orderRepo)
	getUserOrdersHandler := queries.NewGetUserOrdersHandler(orderRepo)

	orderingHandler := orderinghttp.NewOrderingHandler(
		checkoutHandler,
		cancelOrderHandler,
		simulatePaymentHandler,
		getOrderByIDHandler,
		getUserOrdersHandler,
		logger,
	)
	orderingHandler.RegisterRoutes(r)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("ordering service listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutdown signal received, draining connections...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	cancel() // stop saga consumers

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", slog.String("error", err.Error()))
	}

	logger.Info("ordering service stopped")
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

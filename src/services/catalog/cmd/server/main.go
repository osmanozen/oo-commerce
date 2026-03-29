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
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/messaging"
	bbmiddleware "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/middleware"
	cataloghttp "github.com/osmanozen/oo-commerce/src/services/catalog/internal/adapters/http"
	catalogpersistence "github.com/osmanozen/oo-commerce/src/services/catalog/internal/adapters/persistence"
	"github.com/osmanozen/oo-commerce/src/services/catalog/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/catalog/internal/application/queries"
)

func main() {
	// ─── Logger ──────────────────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("catalog service starting",
		slog.String("version", "1.0.0"),
		slog.String("go_version", "1.26.1"),
	)

	// ─── Configuration ──────────────────────────────────────────────────
	port := envOrDefault("PORT", "8081")
	kafkaBrokers := []string{envOrDefault("KAFKA_BROKERS", "localhost:9092")}
	databaseURL := envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ocommerce?sslmode=disable")

	// ─── Kafka Producer ─────────────────────────────────────────────────
	kafkaCfg := messaging.DefaultKafkaProducerConfig(kafkaBrokers)
	kafkaProducer := messaging.NewKafkaProducer(kafkaCfg, logger)
	defer kafkaProducer.Close()

	// ─── Ensure Kafka Topics ────────────────────────────────────────────
	ctx := context.Background()
	catalogTopics := []messaging.TopicConfig{
		{Name: "catalog.product.created", NumPartitions: 6, ReplicationFactor: 1},
		{Name: "catalog.product.updated", NumPartitions: 6, ReplicationFactor: 1},
		{Name: "catalog.product.deleted", NumPartitions: 6, ReplicationFactor: 1},
	}
	if err := messaging.EnsureTopics(ctx, kafkaBrokers[0], catalogTopics, logger); err != nil {
		logger.Warn("failed to ensure kafka topics (may already exist)", slog.String("error", err.Error()))
	}

	// ─── Database + Repositories ────────────────────────────────────────
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

	categoryRepo := catalogpersistence.NewCategoryRepository(pool, logger)
	productRepo := catalogpersistence.NewProductRepository(pool, logger)

	createCategory := commands.NewCreateCategoryHandler(categoryRepo)
	updateCategory := commands.NewUpdateCategoryHandler(categoryRepo)
	deleteCategory := commands.NewDeleteCategoryHandler(categoryRepo)
	getCategories := queries.NewGetCategoriesHandler(categoryRepo)
	getCategoryByID := queries.NewGetCategoryByIdHandler(categoryRepo)

	createProduct := commands.NewCreateProductHandler(productRepo, categoryRepo)
	updateProduct := commands.NewUpdateProductHandler(productRepo, categoryRepo)
	deleteProduct := commands.NewDeleteProductHandler(productRepo)
	updateReviewStats := commands.NewUpdateReviewStatsHandler(productRepo)
	getProducts := queries.NewGetProductsHandler(productRepo)
	getProductByID := queries.NewGetProductByIDHandler(productRepo)

	categoryHandler := cataloghttp.NewCategoryHandler(
		createCategory,
		updateCategory,
		deleteCategory,
		getCategories,
		getCategoryByID,
		logger,
	)
	productHandler := cataloghttp.NewProductHandler(
		createProduct,
		updateProduct,
		deleteProduct,
		updateReviewStats,
		getProducts,
		getProductByID,
		logger,
	)

	// ─── HTTP Router ────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(chimiddleware.RequestID)
	r.Use(bbmiddleware.CorrelationID)
	r.Use(bbmiddleware.RequestLogger(logger))
	r.Use(bbmiddleware.Recovery(logger))
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Compress(5))
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy","service":"catalog"}`)
	})

	categoryHandler.RegisterRoutes(r)
	productHandler.RegisterRoutes(r)

	// ─── HTTP Server ────────────────────────────────────────────────────
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ─── Graceful Shutdown ──────────────────────────────────────────────
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("catalog service listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Block until shutdown signal.
	<-done
	logger.Info("shutdown signal received, draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", slog.String("error", err.Error()))
	}

	logger.Info("catalog service stopped")
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

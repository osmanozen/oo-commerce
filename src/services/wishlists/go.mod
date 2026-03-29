module github.com/osmanozen/oo-commerce/services/wishlists

go 1.26.1

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.1
	github.com/osmanozen/oo-commerce/pkg/buildingblocks v0.0.0
	github.com/shopspring/decimal v1.4.0
)

replace github.com/osmanozen/oo-commerce/pkg/buildingblocks => ../../pkg/buildingblocks

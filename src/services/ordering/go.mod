module github.com/osmanozen/oo-commerce/services/ordering

go 1.26.1

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/looplab/fsm v1.0.2
	github.com/osmanozen/oo-commerce/pkg/buildingblocks v0.0.0
	github.com/shopspring/decimal v1.4.0
)

require (
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/segmentio/kafka-go v0.4.47 // indirect
	github.com/stretchr/testify v1.8.1 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace github.com/osmanozen/oo-commerce/pkg/buildingblocks => ../../pkg/buildingblocks

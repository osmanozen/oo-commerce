# oo-commerce

Go-based e-commerce backend built with domain-oriented microservices, using DDD + CQRS internally and Kafka-driven events with Saga and Outbox patterns for cross-service consistency.

## Architecture
- Microservices architecture (service per business domain)
- DDD style layering (domain, application, adapters)
- CQRS pattern for command/query separation
- Event-driven communication with Kafka
- Saga orchestration for distributed checkout flow
- Outbox pattern for reliable event publishing

## Technologies
- Go (Golang)
- Chi HTTP router
- PostgreSQL
- Kafka
- Traefik (API gateway / reverse proxy)
- Docker and Docker Compose
- Makefile-based local workflows

## API Gateway (Traefik)
- Base URL: `https://localhost`
- Traefik dashboard: `http://localhost:8089/dashboard/`
- API route format: `/api/<service>/...`
- Local TLS certificate files: `infra/traefik/certs/localhost.crt` and `infra/traefik/certs/localhost.key`
- Service health routes via gateway:
	- `GET /catalog/health`
	- `GET /cart/health`
	- `GET /ordering/health`
	- `GET /inventory/health`
	- `GET /profiles/health`
	- `GET /reviews/health`
	- `GET /wishlists/health`
	- `GET /coupons/health`

All `/api/*` service routes are reachable through Traefik on port `443`.
Certificate was generated for `CN=localhost` and imported into `CurrentUser\Root` on Windows for local trust.

## Services

#### Cart Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /cart/health | Health check endpoint (gateway) |
| GET | /api/cart/ | Get current user or guest cart |
| DELETE | /api/cart/ | Clear all items in current cart |
| POST | /api/cart/merge | Merge guest cart into user cart |
| POST | /api/cart/items/ | Add an item to cart |
| PATCH | /api/cart/items/{itemId} | Update quantity of a cart item |
| DELETE | /api/cart/items/{itemId} | Remove an item from cart |

#### Catalog Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /catalog/health | Health check endpoint (gateway) |
| GET | /api/catalog/categories/ | List categories |
| POST | /api/catalog/categories/ | Create a category |
| GET | /api/catalog/categories/{id} | Get category details |
| PUT | /api/catalog/categories/{id} | Update a category |
| DELETE | /api/catalog/categories/{id} | Delete a category |
| GET | /api/catalog/products/ | List and filter products |
| POST | /api/catalog/products/ | Create a product |
| GET | /api/catalog/products/{id} | Get product details |
| PUT | /api/catalog/products/{id} | Update a product |
| DELETE | /api/catalog/products/{id} | Delete a product |
| PATCH | /api/catalog/products/{id}/review-stats | Update product rating stats |

#### Coupons Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /coupons/health | Health check endpoint (gateway) |
| POST | /api/coupons/ | Create a coupon |
| GET | /api/coupons/ | List coupons |
| POST | /api/coupons/validate | Validate coupon against basket |
| GET | /api/coupons/{id} | Get coupon details |
| PUT | /api/coupons/{id} | Update coupon data |
| PATCH | /api/coupons/{id}/status | Enable or disable coupon |

#### Inventory Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /inventory/health | Health check endpoint (gateway) |
| GET | /api/inventory/stock/{productId} | Get stock for one product |
| POST | /api/inventory/stock/{productId}/adjust | Increase or decrease stock |
| POST | /api/inventory/stock/levels | Get stock levels for many products |

#### Ordering Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /ordering/health | Health check endpoint (gateway) |
| POST | /api/ordering/checkout | Create order from checkout request |
| GET | /api/ordering/orders/ | List current user orders |
| GET | /api/ordering/orders/{orderId} | Get order details |
| POST | /api/ordering/orders/{orderId}/cancel | Cancel an order |
| POST | /api/ordering/orders/{orderId}/pay | Simulate payment for an order |

#### Profiles Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /profiles/health | Health check endpoint (gateway) |
| GET | /api/profiles/me/ | Get current user profile |
| PUT | /api/profiles/me/ | Update profile info |
| POST | /api/profiles/me/avatar | Upload avatar metadata |
| DELETE | /api/profiles/me/avatar | Remove avatar |
| POST | /api/profiles/me/addresses | Add a new address |
| PUT | /api/profiles/me/addresses/{addressId} | Update an address |
| DELETE | /api/profiles/me/addresses/{addressId} | Delete an address |
| PATCH | /api/profiles/me/addresses/{addressId}/default | Set default address |

#### Reviews Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /reviews/health | Health check endpoint (gateway) |
| GET | /api/reviews/products/{productId}/ | List reviews for a product |
| GET | /api/reviews/products/{productId}/mine | Get current user review |
| GET | /api/reviews/products/{productId}/can-review | Check if user can review |
| POST | /api/reviews/products/{productId}/ | Create a review |
| PUT | /api/reviews/{reviewId} | Update a review |
| DELETE | /api/reviews/{reviewId} | Delete a review |

#### Wishlists Service
| Method | Path | Description |
| --- | --- | --- |
| GET | /wishlists/health | Health check endpoint (gateway) |
| GET | /api/wishlist/ | Get wishlist items |
| GET | /api/wishlist/count | Get wishlist item count |
| GET | /api/wishlist/product-ids | Get wishlist product IDs only |
| POST | /api/wishlist/{productId} | Add product to wishlist |
| DELETE | /api/wishlist/{productId} | Remove product from wishlist |
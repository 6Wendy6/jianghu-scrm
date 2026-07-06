.PHONY: dev dev-backend dev-backend-db dev-db migrate seed test-backend test-db e2e-cleanup-dry-run e2e-cleanup-apply build status

dev:
	npm run dev

dev-backend:
	npm run dev:backend

dev-backend-db:
	npm run dev:backend:db

dev-db:
	npm run db:up

migrate:
	npm run db:migrate

seed:
	npm run db:seed

status:
	npm run db:status

test-backend:
	npm run test:backend

test-db:
	npm run test:backend:db

e2e-cleanup-dry-run:
	npm run e2e:cleanup:dry-run

e2e-cleanup-apply:
	npm run e2e:cleanup:apply

build:
	npm run build

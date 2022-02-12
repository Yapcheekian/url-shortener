export PGPASS ?= mysecretpassword
export PGUSER ?= dcard_test
export PGDB ?= dcard_test
export PGPORT ?= 5432
export PGVERSION ?= 14
export RDPORT ?= 6379
export RDVERSION ?= 6

all: db redis seed

db:
	docker run --name postgres -e POSTGRES_PASSWORD=${PGPASS} -e POSTGRES_USER=${PGUSER} -e POSTGRES_DB=${ui_test} -p ${PGPORT}:${PGPORT}  -d postgres:${PGVERSION}

redis:
	docker run --name redis -p ${RDPORT}:${RDPORT} -d redis:${RDVERSION}

seed:
	cd ./bootstrap && go run bootstrap.go

test:
	go test -race -cover ./...

clean:
	docker rm -f postgres redis

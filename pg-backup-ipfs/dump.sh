source .env

docker exec -e PGPASSWORD=world123 dumper_pg pg_dump -U world -d world-db | go run main.go
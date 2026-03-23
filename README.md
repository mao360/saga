up

docker compose --env-file .env.infra -f docker-compose.infra.yaml up -d
cd order && docker compose up -d --build
cd inventory && docker compose up -d --build
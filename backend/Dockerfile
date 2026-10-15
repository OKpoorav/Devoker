FROM python:3.12-slim AS build
WORKDIR /app
RUN python -m venv /venv
COPY requirements.txt .
RUN /venv/bin/pip install --no-cache-dir -r requirements.txt

FROM python:3.12-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PATH=/venv/bin:$PATH
COPY --from=build /venv /venv
COPY alembic.ini ./
COPY alembic ./alembic
COPY app ./app
# Numeric, so Kubernetes runAsNonRoot can verify it without reading /etc/passwd.
USER 10001:10001
EXPOSE 8000
# Migrations run on every start; replicas take turns on a Postgres advisory lock (alembic/env.py).
# No access log: event-stream URLs carry each player's secret token.
CMD ["sh", "-c", "alembic upgrade head && exec uvicorn app.main:app --host 0.0.0.0 --port 8000 --proxy-headers --no-access-log"]

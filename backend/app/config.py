from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    database_url: str = "postgresql+psycopg://devoker:devoker@localhost:5432/devoker"
    starting_chips: int = 1000
    small_blind: int = 10
    big_blind: int = 20
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")


settings = Settings()

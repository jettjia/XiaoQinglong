"""FastAPI main application for runner-langchain."""

import os
import structlog
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.routes import router
from app.api.schemas import HealthResponse

logger = structlog.get_logger()

PORT = int(os.getenv("RUNNER_PORT", "18088"))


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    logger.info("runner_langchain.starting", port=PORT)
    yield
    logger.info("runner_langchain.stopping")


app = FastAPI(
    title="Runner-LangChain",
    description="LangChain-based agent runner with hermes-agent style delegation",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(router)


@app.get("/", response_model=HealthResponse)
async def root():
    """Root endpoint."""
    return HealthResponse()


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=PORT,
        reload=False,
    )

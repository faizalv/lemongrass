from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from markitdown import MarkItDown
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

model: SentenceTransformer | None = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    global model
    model = SentenceTransformer("intfloat/e5-base", cache_folder="/app/models")
    yield

app = FastAPI(lifespan=lifespan)

class EmbedRequest(BaseModel):
    text: str

class ConvertRequest(BaseModel):
    path: str

@app.post("/embed")
def embed(req: EmbedRequest):
    if model is None:
        raise HTTPException(status_code=503, detail="model not ready")
    embedding = model.encode(req.text, normalize_embeddings=True).tolist()
    return {"embedding": embedding}

@app.post("/convert")
def convert(req: ConvertRequest):
    try:
        md = MarkItDown()
        result = md.convert(req.path)
        return {"markdown": result.text_content}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
def health():
    return {"status": "ok"}

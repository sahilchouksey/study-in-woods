"""
Simple OCR Service
Takes a PDF (binary or URL) and returns extracted text
NO database, NO job tracking, NO webhooks - just OCR!
"""

import io
import tempfile
import logging
from typing import Optional
from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, HttpUrl
import requests

# Lazy import for Docling (imported only when needed)
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Simple OCR Service", version="1.0.0")

# Global converter instance (lazy loaded)
converter = None


def get_converter():
    """Lazy load Docling converter with optimized settings for exam papers"""
    global converter
    if converter is None:
        logger.info("Initializing Docling converter...")
        from docling.document_converter import DocumentConverter
        from docling.datamodel.pipeline_options import PdfPipelineOptions

        pipeline_options = PdfPipelineOptions()
        pipeline_options.do_ocr = True
        pipeline_options.do_table_structure = True
        # Higher image scale for better OCR quality on scanned documents
        # Default is 1.0, we use 2.0 for better text recognition on exam papers
        pipeline_options.images_scale = 2.0

        converter = DocumentConverter(
            allowed_formats=None,
            format_options={"PDF": pipeline_options},  # type: ignore
        )
        logger.info("Docling converter ready with enhanced settings (images_scale=2.0)")
    return converter


class URLRequest(BaseModel):
    """Request model for URL-based OCR"""

    url: HttpUrl


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "ocr"}


@app.post("/ocr/file")
async def ocr_from_file(file: UploadFile = File(...)):
    """
    Extract text from uploaded PDF file

    Args:
        file: PDF file (multipart/form-data)

    Returns:
        {"text": "extracted text content", "page_count": 10}
    """
    try:
        # Validate PDF
        if not file.filename.lower().endswith(".pdf"):
            raise HTTPException(status_code=400, detail="Only PDF files are supported")

        # Read PDF bytes
        pdf_bytes = await file.read()
        logger.info(
            f"Processing uploaded PDF: {file.filename} ({len(pdf_bytes)} bytes)"
        )

        # Process OCR
        text, page_count = process_pdf_bytes(pdf_bytes)

        return JSONResponse(
            {"text": text, "page_count": page_count, "filename": file.filename}
        )

    except Exception as e:
        logger.error(f"OCR failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/ocr/url")
async def ocr_from_url(request: URLRequest):
    """
    Extract text from PDF at given URL

    Args:
        url: URL to PDF file

    Returns:
        {"text": "extracted text content", "page_count": 10}
    """
    try:
        url = str(request.url)
        logger.info(f"Downloading PDF from URL: {url}")

        # Download PDF
        response = requests.get(url, timeout=60)
        response.raise_for_status()

        if "application/pdf" not in response.headers.get("Content-Type", ""):
            raise HTTPException(
                status_code=400, detail="URL does not point to a PDF file"
            )

        pdf_bytes = response.content
        logger.info(f"Downloaded {len(pdf_bytes)} bytes")

        # Process OCR
        text, page_count = process_pdf_bytes(pdf_bytes)

        return JSONResponse({"text": text, "page_count": page_count, "source_url": url})

    except requests.RequestException as e:
        logger.error(f"Failed to download PDF: {e}")
        raise HTTPException(status_code=400, detail=f"Failed to download PDF: {str(e)}")
    except Exception as e:
        logger.error(f"OCR failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


def process_pdf_bytes(pdf_bytes: bytes) -> tuple[str, int]:
    """
    Process PDF bytes with Docling OCR

    Args:
        pdf_bytes: PDF file as bytes

    Returns:
        Tuple of (extracted_text, page_count)
    """
    tmp_path = None
    try:
        # Get converter instance
        conv = get_converter()

        # Save to temp file (Docling needs file path)
        with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as tmp_file:
            tmp_file.write(pdf_bytes)
            tmp_path = tmp_file.name

        # Convert with Docling
        logger.info("Converting PDF with Docling OCR...")
        result = conv.convert(tmp_path)

        # Extract text and metadata
        text = result.document.export_to_markdown()
        page_count = (
            len(result.document.pages) if hasattr(result.document, "pages") else 0
        )

        logger.info(f"OCR completed: {page_count} pages, {len(text)} characters")

        return text, page_count

    finally:
        # Cleanup temp file
        if tmp_path:
            import os

            try:
                os.unlink(tmp_path)
            except:
                pass


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8081)

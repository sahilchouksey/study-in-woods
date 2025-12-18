"""
OCR Service with PaddleOCR

Environment variables:
- OCR_DPI: DPI for PDF rendering (default: 300)
"""

import os
import re
import gc
import logging
from typing import Tuple
from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, HttpUrl
import requests

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="OCR Service", version="2.0.0")

# Configuration
OCR_DPI = int(os.getenv("OCR_DPI", "300"))

# Global instance (lazy loaded)
_paddle_ocr = None


def get_paddle_ocr():
    """Lazy load PaddleOCR with optimized settings for scanned exam papers"""
    global _paddle_ocr
    if _paddle_ocr is None:
        logger.info("Initializing PaddleOCR with optimized settings...")
        from paddleocr import PaddleOCR
        
        _paddle_ocr = PaddleOCR(
            lang='en',
            device='cpu',
            ocr_version='PP-OCRv4',
            use_doc_orientation_classify=False,
            use_doc_unwarping=False,
            use_textline_orientation=False,
            # Improved detection settings (lowered thresholds to catch all text)
            text_det_box_thresh=0.3,
            text_det_thresh=0.2,
            text_det_unclip_ratio=2.0,
            text_det_limit_side_len=2048,
            text_rec_score_thresh=0.5,
        )
        logger.info("PaddleOCR ready (PP-OCRv4, optimized detection)")
    return _paddle_ocr


def preprocess_image_for_paddle(img):
    """Apply preprocessing to improve OCR detection on scanned documents"""
    import cv2
    import numpy as np
    
    if hasattr(img, 'convert'):
        img = np.array(img.convert('RGB'))
    
    gray = cv2.cvtColor(img, cv2.COLOR_RGB2GRAY)
    clahe = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8, 8))
    enhanced = clahe.apply(gray)
    denoised = cv2.medianBlur(enhanced, 3)
    rgb = cv2.cvtColor(denoised, cv2.COLOR_GRAY2RGB)
    bordered = cv2.copyMakeBorder(
        rgb, 50, 50, 50, 50, 
        cv2.BORDER_CONSTANT, value=(255, 255, 255)
    )
    return bordered


def filter_garbage_text(text: str) -> str:
    """Filter out Hindi/non-ASCII and garbage text"""
    if not text or len(text.strip()) < 2:
        return ""
    if any(ord(c) > 127 for c in text):
        return ""
    if re.match(r'^[^a-zA-Z0-9\[\]\(\){}]*$', text):
        return ""
    return text.strip()


def merge_text_lines(texts_with_positions: list) -> str:
    """Merge OCR text fragments into coherent paragraphs."""
    if not texts_with_positions:
        return ""
    
    sorted_texts = sorted(texts_with_positions, key=lambda x: (x['y'], x['x']))
    
    lines = []
    current_line = []
    current_y = None
    y_threshold = 20
    
    for item in sorted_texts:
        if current_y is None:
            current_y = item['y']
            current_line.append(item['text'])
        elif abs(item['y'] - current_y) < y_threshold:
            current_line.append(item['text'])
        else:
            lines.append(' '.join(current_line))
            current_line = [item['text']]
            current_y = item['y']
    
    if current_line:
        lines.append(' '.join(current_line))
    
    return '\n'.join(lines)


def process_pdf_bytes(pdf_bytes: bytes) -> Tuple[str, int]:
    """Process PDF with PaddleOCR"""
    from pdf2image import convert_from_bytes
    
    ocr = get_paddle_ocr()
    
    logger.info(f"Converting PDF to images at {OCR_DPI} DPI...")
    images = convert_from_bytes(pdf_bytes, dpi=OCR_DPI)
    page_count = len(images)
    logger.info(f"Found {page_count} pages")
    
    all_pages_text = []
    
    for i, image in enumerate(images):
        page_num = i + 1
        logger.info(f"Processing page {page_num}/{page_count}...")
        
        preprocessed = preprocess_image_for_paddle(image)
        result = ocr.predict(preprocessed)
        
        texts_with_positions = []
        
        if result:
            for item in result:
                if 'rec_texts' in item and 'rec_scores' in item and 'dt_polys' in item:
                    for text, score, poly in zip(
                        item['rec_texts'], 
                        item['rec_scores'],
                        item['dt_polys']
                    ):
                        text = filter_garbage_text(text)
                        if text and score > 0.5:
                            y = float(poly[0][1])
                            x = float(poly[0][0])
                            texts_with_positions.append({
                                'text': text,
                                'score': score,
                                'x': x,
                                'y': y
                            })
        
        page_text = merge_text_lines(texts_with_positions)
        
        if page_text:
            all_pages_text.append(f"--- Page {page_num} ---\n{page_text}")
        
        gc.collect()
    
    full_text = '\n\n'.join(all_pages_text)
    logger.info(f"PaddleOCR completed: {page_count} pages, {len(full_text)} characters")
    
    return full_text, page_count


# =============================================================================
# API Endpoints
# =============================================================================

class URLRequest(BaseModel):
    """Request model for URL-based OCR"""
    url: HttpUrl


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy", 
        "service": "ocr",
        "engine": "paddle",
        "dpi": OCR_DPI
    }


@app.post("/ocr/file")
async def ocr_from_file(file: UploadFile = File(...)):
    """Extract text from uploaded PDF file"""
    try:
        if not file.filename.lower().endswith(".pdf"):
            raise HTTPException(status_code=400, detail="Only PDF files are supported")

        pdf_bytes = await file.read()
        logger.info(f"Processing uploaded PDF: {file.filename} ({len(pdf_bytes)} bytes)")

        text, page_count = process_pdf_bytes(pdf_bytes)

        return JSONResponse({
            "text": text, 
            "page_count": page_count, 
            "filename": file.filename,
            "engine": "paddle"
        })

    except Exception as e:
        logger.error(f"OCR failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/ocr/url")
async def ocr_from_url(request: URLRequest):
    """Extract text from PDF at given URL"""
    try:
        url = str(request.url)
        logger.info(f"Downloading PDF from URL: {url}")

        response = requests.get(url, timeout=60)
        response.raise_for_status()

        content_type = response.headers.get("Content-Type", "")
        if "application/pdf" not in content_type and not url.lower().endswith('.pdf'):
            raise HTTPException(
                status_code=400, detail="URL does not point to a PDF file"
            )

        pdf_bytes = response.content
        logger.info(f"Downloaded {len(pdf_bytes)} bytes")

        text, page_count = process_pdf_bytes(pdf_bytes)

        return JSONResponse({
            "text": text, 
            "page_count": page_count, 
            "source_url": url,
            "engine": "paddle"
        })

    except requests.RequestException as e:
        logger.error(f"Failed to download PDF: {e}")
        raise HTTPException(status_code=400, detail=f"Failed to download PDF: {str(e)}")
    except Exception as e:
        logger.error(f"OCR failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8081)

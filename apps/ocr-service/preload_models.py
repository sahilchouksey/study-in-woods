"""
Pre-download PaddleOCR models during Docker build.
This avoids downloading models at runtime.
"""
import logging
import os

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def preload_paddleocr():
    """Download PaddleOCR detection and recognition models"""
    logger.info("Downloading PaddleOCR models...")
    from paddleocr import PaddleOCR
    
    ocr = PaddleOCR(
        lang='en',
        device='cpu',
        ocr_version='PP-OCRv4',
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=False,
        text_det_box_thresh=0.3,
        text_det_thresh=0.2,
        text_det_unclip_ratio=2.0,
        text_det_limit_side_len=2048,
        text_rec_score_thresh=0.5,
    )
    
    # Run a minimal prediction to ensure models are fully loaded
    import numpy as np
    dummy_image = np.ones((100, 300, 3), dtype=np.uint8) * 255
    try:
        ocr.predict(dummy_image)
    except Exception as e:
        logger.warning(f"Dummy prediction had issues (expected): {e}")
    
    logger.info("PaddleOCR models downloaded successfully")
    del ocr


def get_total_size(paths):
    """Get total size of files in given paths"""
    total = 0
    for base_path in paths:
        if os.path.exists(base_path):
            for root, dirs, files in os.walk(base_path):
                for f in files:
                    try:
                        total += os.path.getsize(os.path.join(root, f))
                    except:
                        pass
    return total


if __name__ == "__main__":
    logger.info("=" * 60)
    logger.info("Pre-loading PaddleOCR models for Docker image...")
    logger.info("=" * 60)
    
    home = os.path.expanduser("~")
    model_paths = [
        os.path.join(home, ".paddleocr"),
        os.path.join(home, ".paddlex"),
        os.path.join(home, ".cache"),
    ]
    
    preload_paddleocr()
    
    total_size = get_total_size(model_paths)
    
    logger.info("=" * 60)
    logger.info(f"Total models size: {total_size / 1024 / 1024:.1f} MB")
    
    for p in model_paths:
        if os.path.exists(p):
            size = get_total_size([p])
            logger.info(f"  {p}: {size/1024/1024:.1f} MB")
    
    logger.info("Models pre-loaded successfully!")
    logger.info("=" * 60)

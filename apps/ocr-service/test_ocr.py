"""
Test script for OCR service
Reads PDF from data/ directory and saves OCR output to output/ directory
"""

import os
import sys
import json
import requests
from pathlib import Path

# Configuration
OCR_SERVICE_URL = "http://localhost:8081"
DATA_DIR = Path(__file__).parent / "data"
OUTPUT_DIR = Path(__file__).parent / "output"
PDF_FILE = DATA_DIR / "mca-301-data-mining-dec-2024.pdf"

# Colors for terminal output
GREEN = "\033[92m"
RED = "\033[91m"
YELLOW = "\033[93m"
BLUE = "\033[94m"
RESET = "\033[0m"


def log_info(msg):
    print(f"{BLUE}[INFO]{RESET} {msg}")


def log_success(msg):
    print(f"{GREEN}[SUCCESS]{RESET} {msg}")


def log_error(msg):
    print(f"{RED}[ERROR]{RESET} {msg}")


def log_warning(msg):
    print(f"{YELLOW}[WARNING]{RESET} {msg}")


def check_service_health():
    """Check if OCR service is running"""
    try:
        response = requests.get(f"{OCR_SERVICE_URL}/health", timeout=5)
        if response.status_code == 200:
            log_success("OCR service is healthy!")
            return True
        else:
            log_error(f"OCR service returned status {response.status_code}")
            return False
    except requests.exceptions.ConnectionError:
        log_error("Cannot connect to OCR service. Is it running?")
        log_info("Start it with: cd apps/ocr-service && python main.py")
        return False
    except Exception as e:
        log_error(f"Health check failed: {e}")
        return False


def test_ocr_from_file():
    """Test OCR by uploading a PDF file"""
    log_info(f"Testing OCR with file: {PDF_FILE.name}")

    # Check if file exists
    if not PDF_FILE.exists():
        log_error(f"PDF file not found: {PDF_FILE}")
        return False

    log_info(f"File size: {PDF_FILE.stat().st_size / 1024:.2f} KB")

    # Prepare file upload
    try:
        with open(PDF_FILE, "rb") as f:
            files = {"file": (PDF_FILE.name, f, "application/pdf")}

            log_info("Uploading PDF to OCR service...")
            response = requests.post(
                f"{OCR_SERVICE_URL}/ocr/file",
                files=files,
                timeout=300,  # 5 minutes timeout for large PDFs
            )

        if response.status_code == 200:
            result = response.json()

            # Log results
            log_success("OCR processing completed!")
            log_info(f"Page count: {result['page_count']}")
            log_info(f"Text length: {len(result['text'])} characters")
            log_info(f"Text preview (first 200 chars):\n{result['text'][:200]}...")

            # Save output to file
            output_file = OUTPUT_DIR / f"{PDF_FILE.stem}_ocr.json"
            with open(output_file, "w", encoding="utf-8") as f:
                json.dump(result, f, indent=2, ensure_ascii=False)

            log_success(f"Output saved to: {output_file}")

            # Also save just the text
            text_file = OUTPUT_DIR / f"{PDF_FILE.stem}_ocr.txt"
            with open(text_file, "w", encoding="utf-8") as f:
                f.write(result["text"])

            log_success(f"Text saved to: {text_file}")

            return True
        else:
            log_error(f"OCR failed with status {response.status_code}")
            log_error(f"Response: {response.text}")
            return False

    except requests.exceptions.Timeout:
        log_error("Request timed out. PDF might be too large or OCR is slow.")
        return False
    except Exception as e:
        log_error(f"Test failed: {e}")
        import traceback

        traceback.print_exc()
        return False


def main():
    """Run OCR tests"""
    print("\n" + "=" * 60)
    print(f"{BLUE}OCR Service Test{RESET}")
    print("=" * 60 + "\n")

    # Ensure output directory exists
    OUTPUT_DIR.mkdir(exist_ok=True)
    log_info(f"Output directory: {OUTPUT_DIR}")

    # Step 1: Check if service is running
    log_info("Step 1: Checking OCR service health...")
    if not check_service_health():
        log_error("OCR service is not running. Please start it first.")
        sys.exit(1)

    print()

    # Step 2: Test OCR
    log_info("Step 2: Testing OCR with PDF file...")
    success = test_ocr_from_file()

    print("\n" + "=" * 60)
    if success:
        log_success("All tests passed! ✓")
    else:
        log_error("Tests failed! ✗")
    print("=" * 60 + "\n")

    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()

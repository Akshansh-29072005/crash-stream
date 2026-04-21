from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
from ultralytics import YOLO
from PIL import Image
import io
import httpx
import asyncio
import os
import datetime

app = FastAPI()

# Configuration
GO_BACKEND_URL = os.getenv("GO_BACKEND_URL", "http://localhost:8080")

# Load your pre-trained YOLO 11x model for accident detection
# Using the specific custom model cloned by the user
try:
    model_path = "/models/weights/epoch61.pt"
    if not os.path.exists(model_path):
        # Fallback for local development or if mount fails
        model_path = "../traffic-accident-detection-yolo11x/weights/epoch61.pt"
    
    print(f"Loading custom YOLOv11x model from {model_path}...")
    model = YOLO(model_path)
    print("YOLO model loaded successfully.")
except Exception as e:
    print(f"Error loading YOLO model: {e}")
    # Fallback to standard standard if custom fails
    try:
        model = YOLO("yolo11x.pt")
        print("Fallback YOLO model loaded.")
    except:
        model = None

async def send_result_to_go_backend(result: dict):
    """Send detection result to Go backend for WebSocket broadcasting"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{GO_BACKEND_URL}/api/detection-result",
                json=result,
                timeout=5.0
            )
            if response.status_code != 200:
                print(f"Failed to send result to Go backend: {response.status_code}")
    except Exception as e:
        print(f"Error sending result to Go backend: {e}")

@app.post("/detect")
async def detect_accidents(file: UploadFile = File(...)):
    if not file:
        raise HTTPException(status_code=400, detail="No file uploaded")

    # Read the image
    contents = await file.read()
    image = Image.open(io.BytesIO(contents))

    # If model is not loaded, return error
    if model is None:
        return JSONResponse(
            status_code=500,
            content={"error": "Model not loaded"}
        )

    # Perform inference
    # YOLO returns a list of Results objects
    results = model(image, conf=0.15)
    
    detection_occurred = False
    max_conf = 0.0
    bbox = None
    
    # Process results (YOLOv11 specific)
    for r in results:
        for box in r.boxes:
            # For accident detection models, class 0 (or similar) is usually 'accident'
            # Here we assume the model output classes indicate accidents.
            conf = float(box.conf[0])
            if conf > max_conf:
                max_conf = conf
                detection_occurred = True
                # Get bbox as [x1, y1, x2, y2]
                bbox = box.xyxy[0].tolist()

    result = {
        "accident_detected": detection_occurred,
        "confidence": max_conf,
        "bbox": bbox,
        "timestamp": datetime.datetime.now().isoformat()
    }
    
    print(f">>> [AI LOG] Detection: {'ACCIDENT' if detection_occurred else 'None'} | Conf: {max_conf:.4f}")

    # Send result to Go backend (non-blocking)
    asyncio.create_task(send_result_to_go_backend(result))

    return JSONResponse(content=result)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=5000, reload=True)

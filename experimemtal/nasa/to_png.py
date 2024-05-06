import os
import cv2
import sys
import numpy as np

ROWS = 3601
COLS = 3601
current_dir = os.getcwd()

def process_heightmap_file(file_path):
    # Check file extension
    if not file_path.lower().endswith(('.hgt', '.raw')):
        raise ValueError("Invalid file format. Only .hgt or .raw files are supported.")

    output_path = file_path[:-4] + ".png"

    # Define a color gradient for the heightmap
    color_gradient = np.array([[0, 0, 255],   # Blue
                               [0, 255, 0],   # Green
                               [255, 255, 255]])  # White

    with open(file_path, 'rb') as raw_image:
        # Read the big-endian data
        image = np.fromfile(raw_image, dtype='>u2', count=ROWS * COLS).reshape(COLS, ROWS)

        # Normalize heights to [0, 1] range
        normalized_image = (image - np.min(image)) / (np.max(image) - np.min(image))

        # Interpolate colors based on height
        interpolated_colors = np.zeros((COLS, ROWS, 3), dtype=np.uint8)
        for i in range(3):
            interpolated_colors[:, :, i] = np.interp(normalized_image, np.linspace(0, 1, 3), color_gradient[:, i])

        # Save the image
        cv2.imwrite(output_path, interpolated_colors, [cv2.IMWRITE_PNG_COMPRESSION, 5])

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python script_name.py <input_file>")
        sys.exit(1)

    input_file = sys.argv[1]
    file_path = os.path.join(current_dir, input_file)

    if not os.path.exists(file_path):
        print("Error: Input file does not exist.")
        sys.exit(1)

    try:
        process_heightmap_file(file_path)
        print("Image saved successfully:", file_path[:-4] + ".png")
    except ValueError as e:
        print("Error:", e)

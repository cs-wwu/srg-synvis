import numpy as np
from PIL import Image

min_elevation = -32768
max_elevation = 32767

def read_hgt_file(file_path):
    with open(file_path, 'rb') as f:
        data = f.read()
        # Read as signed 16-bit integers
        elevations = np.frombuffer(data, dtype='>i2').reshape((3601, 3601))
        return elevations

def normalize_elevations(elevations):
    # Normalize to [0, 65535]
    normalized = ((elevations - min_elevation) / (max_elevation - min_elevation) * 65535).astype(np.uint16)
    return normalized

def save_to_png(elevations, output_path):
    normalized_elevations = normalize_elevations(elevations)
    # Convert the array to an image
    img = Image.fromarray(normalized_elevations, 'I;16')
    img.save(output_path)

def read_png(file_path):
    # Load the PNG image
    img = Image.open(file_path)
    # Ensure it's in 'I;16' mode
    img = img.convert('I;16')
    # Extract pixel values as a NumPy array
    pixel_values = np.array(img, dtype=np.uint16)
    return pixel_values

def denormalize_elevations(pixel_values, min_elevation, max_elevation):
    # Denormalize from [0, 65535] to [min_elevation, max_elevation]
    denormalized = ((pixel_values / 65535) * (max_elevation - min_elevation) + min_elevation).astype(np.int16)
    return denormalized

def write_hgt_file(elevations, output_path):
    with open(output_path, 'wb') as f:
        # Write as signed 16-bit integers in big-endian format
        f.write(elevations.astype('>i2').tobytes())


hgt_file = 'coords/n48w123/n48w123.hgt'
png_file = 'output.png'

elevations = read_hgt_file(hgt_file)
save_to_png(elevations, png_file)

output_hgt_file = 'output.hgt'
pixel_values = read_png(png_file)
elevations = denormalize_elevations(pixel_values, min_elevation, max_elevation)
write_hgt_file(elevations, hgt_file)

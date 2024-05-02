// Converting an .hgt file of int16_t to a .raw file of uint16_t
// Cooper Morgan
// This was made as an attempt to render heightmaps in Godot's hTerrain

#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <sys/types.h>

#define ROWS 3601
#define COLS 3601
int TOTAL = ROWS * COLS;

int main(int argc, char* argv[]) {
    if (argc != 3) {
        printf("./%s <input.hgt> <output.raw>\n", argv[0]);
        return 1;
    }

    char* input_name = argv[1];
    char* output_name = argv[2];

    FILE *input;
    FILE *output;
    if ((input = fopen(input_name, "r")) == NULL) {
        fprintf(stderr, "Unable to open: %s\n", input_name);
        return 1;
    }
    if ((output = fopen(output_name, "w")) == NULL) {
        fprintf(stderr, "Unable to open: %s\n", output_name);
        return 1;
    }

    // malloc large enough buffers to hold both the input and output files
    int16_t* input_buf = (int16_t*)malloc(TOTAL * sizeof(int16_t));
    uint16_t* output_buf = (uint16_t*)malloc(TOTAL * sizeof(uint16_t));
    if (input_buf == NULL || output_buf == NULL) {
        fprintf(stderr, "Memory allocation failed\n");
        return 1;
    }

    if (fread(input_buf, sizeof(int16_t), TOTAL, input) != TOTAL) {
        fprintf(stderr, "Could not read whole file: %s\n", input_name);
        return 1;
    }

    // Convert everything from the input file to uint16_t
    for (int i = 0; i < TOTAL; i++) {
        output_buf[i] = (uint16_t) 32768 + (uint16_t) input_buf[i];
    }

    if (fwrite(output_buf, sizeof(uint16_t), TOTAL, output) != TOTAL) {
        fprintf(stderr, "Could not write to file: %s\n", output_name);
        return 1;
    }

    if (fclose(input) != 0) {
        fprintf(stderr, "Error closing file: %s\n", input_name);
        return 1;
    }

    if (fclose(output) != 0) {
        fprintf(stderr, "Error closing file: %s\n", output_name);
        return 1;
    }
    free(input_buf);
    free(output_buf);
    return 0;
}

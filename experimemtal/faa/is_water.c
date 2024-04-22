#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>

int main(int argc, char** argv) {
    /* Return 0 if any water is found */
    if (argc != 2) {
        printf("Usage: %s <coord.swb>\n", argv[0]);
        return 2;
    }

    /* Check that the file exists */
    FILE *fp = fopen(argv[1], "rb");
    if (fp == NULL) {
        printf("Error: File %s not found.\n", argv[1]);
        return 2;
    }

    /* The file is 3601 by 3601 unsigned 1 byte ints.
       Read each uint_8, if the u_int8 == 0, return 0. */
    uint8_t pixel;
    while (fread(&pixel, sizeof(uint8_t), 1, fp) == 1) {
        if (pixel == 0) {
            printf("Water found.\n");
            fclose(fp);
            return 0;
        }
    }

    fclose(fp);

    /* Return 1 if no 0 is found. */
    printf("No water found.\n");
    return 1;
}

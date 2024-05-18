#!/bin/bash
# Cooper Morgan
# Use zip to compress files after processing them

create_dir() {
    if [ ! -d "$1" ]; then
        mkdir -p "$1"
    fi
}

unzip_file() {
    unzip -q "$1" -d "$2"
}

remove_files() {
    rm -rf "$1"
}

# Verify that there are zip files
if [ -z "$(find . -maxdepth 1 -type f -name '*.zip')" ]; then
    echo "No zip files found."
    exit 1
fi

# Verify that the coords directory exists
if [ ! -d "coords" ]; then
    create_dir "coords"
    echo "Created the coords directory"
fi

if [ ! -f "convert" ]; then
    gcc -o convert convert.c
    echo "Compiled conert.c to convert"
fi

for file in *.zip; do
    coord="${file#NASADEM_HGT_}"     # Remove NASADEM_HGT_
    coord="${coord%.zip}"            # Remove .zip

    # Unzip the data and remove the zip files
    create_dir "coords/$coord"

    unzip_file "$file" "coords/$coord"

    # remove_files "$file"

    remove_files "coords/$coord/$coord.num"

    ./convert "coords/$coord/$coord.hgt" "coords/$coord/$coord.raw"
    echo "$coord processed"

    # Re-compress the data and remove the old data
    # zip -q -r "coords/$coord" "coords/$coord"

    # echo "Zipped coords/$coord"

    # rm -rf "coords/$coord"

done

rm convert
echo "Processed all zip files."
echo "coords/ KB used:"
du -BK coords/

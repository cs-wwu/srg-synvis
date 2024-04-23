#!/bin/bash
# Cooper Morgan
# Unfinished, removed is_water, haven't begun compress_swb
# At this point I'll use zip

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

for file in *.zip; do
    coord="${file#NASADEM_HGT_}"     # Remove NASADEM_HGT_
    coord="${coord%.zip}"            # Remove .zip

    unzip_file "$file" "coords/$coord"

    remove_files "$file"

    remove_files "coords/$coord/$coord.num"

    zip -q -r "coords/$coord.zip" . -i "coord/$coord"

    rm -rf "coords/$coord"

    echo "Zipped coords/$coord"
done

echo "Processed all zip files."

du -BK coords/

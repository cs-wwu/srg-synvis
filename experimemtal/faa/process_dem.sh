#!/bin/bash

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

if [ ! -d "coords" ]; then
    create_dir "coords"
fi

for file in *.zip; do
    coord="${file#NASADEM_HGT_}"     # Remove NASADEM_HGT_
    coord="${coord%.zip}"            # Remove .zip

    unzip_file "$file" "coords/$coord"

    remove_files "$file"

    remove_files "coords/$coord/*.num"

    if  ! ./is_water "coords/$coord/$coord.swb"; then
        remove_files "coord/$coord/*"
        echo "$coord is all water, removing"
    else
        echo "$coord not water."
    fi    
done

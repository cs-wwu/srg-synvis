# Cooper Morgan
#
# Converts .dat/.DAT files to .pkl files for later use
# This is done using a regular expression match per line of the file
# It then converts the data in each line into an array of data to be inputted
# into a pandas DataFrame to be saved into a pickle file for easier use later

import os
import sys
import pandas as pd
import re

NUM_HEADER_ROWS = 4

# Dat zip file available for download here:
# https://www.faa.gov/air_traffic/flight_info/aeronav/digital_products/dof/
#
# User Guide:
# https://www.faa.gov/air_traffic/flight_info/aeronav/digital_products/dof/media/DOF_README_09-03-2019.pdf
# 
# OAS number
# Verification
# Country
# State                             (Optional)
# City
# Lat: Deg Min Sec
# Lon: Deg Min Sec
# Obstacle Type
# AGL   (Above Ground Level)
# AMSL  (Above Mean Sea Level)      (Can be negative)
# Lighting type                     (Optional)
# Horizontal Accuracy               (Optional)
# Vertical Accuracy                 (Optional)
# Mark Indicator                    (Optional)
# FAA Study Number                  (Optional)
# Action
# Julian Date (4 for the year, 3 for days since beginning of year)
pattern = r'([\d\w]{2}-\d{6})\s+'   \
          r'([OU])\s+'              \
          r'(\w{2})\s+'             \
          r'(\w{2})?\s+'            \
          r'(.*?)\s+'               \
          r'(\d{2})\s+'             \
          r'(\d{2})\s+'             \
          r'(\d{2}.\d{2}[SN])\s+'   \
          r'(\d{3})\s+'             \
          r'(\d{2})\s+'             \
          r'(\d{2}.\d{2}[WE])\s+'   \
          r'(.*?)\s+'               \
          r'(\d)\s+'                \
          r'(\d{5})\s+'             \
          r'([-\d]{5})\s+'          \
          r'(\w)?\s+'               \
          r'(\d)?\s+'               \
          r'(\w)?\s+'               \
          r'(\w)?\s+'               \
          r'(\w+)?\s+'              \
          r'([AC])\s+'              \
          r'(\d{7})\s+'

def main(argv):
    if len(argv) < 2 or len(argv) > 3:
        print(f"Usage: python3 {argv[0]} <input_file.dat> [<output_file.pkl>]")
        exit(1)

    dat_file = argv[1]

    if not os.path.exists(dat_file):
        print(f"Error: {dat_file} does not exist.")
        exit(1)

    # If no output file is specified, save it next to the dat_file with .pkl
    if len(argv) == 3:
        output_file = argv[2]
    else:
        output_file = dat_file.replace('.dat', '.pkl').replace('.DAT', '.pkl')

    # Verify that the dat_file exists
    if not os.path.exists(dat_file):
        print(f"Error: {dat_file} not found.")
        exit(1)
    
    with open(dat_file, 'r', encoding='latin-1') as file:
        page_text = file.read()

    lines = page_text.split('\n')
    # Remove the header rows from the lines
    lines = lines[NUM_HEADER_ROWS:-1]

    data = []
    for line in lines:
        match = re.match(pattern, line)
        if match:
            values = [
                match.group(1),   # OAS#
                match.group(2),   # V
                match.group(3),   # CO
                match.group(4),   # ST
                match.group(5),   # CITY
                match.group(6),   # LAT_DEG
                match.group(7),   # LAT_MIN
                match.group(8),   # LAT_SEC
                match.group(9),   # LON_DEG
                match.group(10),  # LON_MIN
                match.group(11),  # LON_SEC
                match.group(12),  # OBSTACLE
                match.group(13),  # QUANTITY
                match.group(14),  # AGL
                match.group(15),  # AMSL
                match.group(16),  # LT
                match.group(17),  # H_ACC
                match.group(18),  # V_ACC
                match.group(19),  # MAR
                match.group(20),  # FAA_STUDY
                match.group(21),  # ACTION
                match.group(22)   # JDATE
            ]
            data.append(values)
        else:
            # If the line doesn't match the pattern, split it and append as is
            print(f"Warning: could not find match on {line}\n")
            values = line.split()
            data.append(values)

    columns = ['OAS', 'V', 'CO', 'ST', 'CITY', 'LAT_DEG', 'LAT_MIN', 'LAT_SEC',
               'LON_DEG', 'LON_MIN', 'LON_SEC', 'OBSTACLE', 'QUANTITY', 'AGL',
               'AMSL', 'LT', 'H_ACC', 'V_ACC', 'MAR', 'FAA_STUDY', 'ACTION', 'JDATE']
    
    # Create a dataframe with the header columns and found data
    df = pd.DataFrame(data, columns=columns)
    df.to_pickle(output_file)

    print(f"Successfully Saved df to {output_file}")
    print(f"Found {len(df)} rows in {dat_file}")

if __name__ == "__main__":
    main(sys.argv)

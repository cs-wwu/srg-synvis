# Scrapes osm data based on query.txt
# Cooper and Kyle

import requests
import json

url = 'https://overpass.kumi.systems/api/interpreter'
query_file = 'query.txt'
osm_file = 'osm.json'

with open(query_file, 'r') as file:
    query_text = file.read()

r = requests.post(url, data=query_text)
if r.status_code != 200:
    print(f"Error, response returned {r.status_code}")
    exit(1)

json_response = r.json()

with open(osm_file, 'w') as file:
    json.dump(json_response, file, indent=4)
    print(f"Succesfully wrote to {osm_file}")

from sense_hat import SenseHat
import time

sense = SenseHat()

def main():
	while 1:
		orientation = sense.get_orientation_radians()
		sense_data = []
		sense_data.append(orientation["yaw"])
		sense_data.append(orientation["pitch"])
		sense_data.append(orientation["roll"])
		print(sense_data, flush=True)

if __name__ == "__main__":
    main()


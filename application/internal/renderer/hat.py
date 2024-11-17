from sense_hat import SenseHat
import time

sense = SenseHat()

def main():
	orientation = sense.get_orientation_radians()
	sense_data = []
	sense_data.append(orientation["yaw"])
	sense_data.append(orientation["pitch"])
	sense_data.append(orientation["roll"])
	sense_data.append(0)
	print(sense_data)

if __name__ == "__main__":
	main()

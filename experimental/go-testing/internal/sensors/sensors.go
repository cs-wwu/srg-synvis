package sensors

// sudo apt update && sudo apt full-updgrade
// sudo apt install sense-hat
// sudo reboot

// sudo apt update
// sudo apt install octave -y
// cd
// cp /usr/share/librtimulib-utils/RTEllipsoidFit ./ -a
// cd RTEllipsoidFit
// RTIMULibCal
// m

// rm ~/.config/sense_hat/RTIMULib.ini
// sudo cp RTIMULib.ini /etc

// python code example for senseHAT
/*
from sense_hat import SenseHat
sh = SenseHat()
sh.show_message("Hello", scroll_speed=(0.06))
while True:
	x, y, z = sh.get_accelerometer_raw().values()
	x = abs(x)
	y = abs(y)
	z = abs(z)
*/
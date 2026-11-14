from sense_hat import SenseHat
import time

sense = SenseHat()

# IMU fusion inputs: (compass, gyroscope, accelerometer).
# Default is all three on. The compass is what makes yaw noisy; leave it on
# for an absolute heading. To test whether the magnetometer is the source of
# the rocking, try (False, True, True): yaw will then drift slowly instead
# of wobbling.
# sense.set_imu_config(False, True, True)

# Poll at a fixed rate. The old loop had no delay, so it re-read the same
# IMU sample thousands of times per second (pegging a Pi core and flooding
# the Go side with duplicate lines).
POLL_SECONDS = 0.01  # ~100 Hz


def main():
    next_t = time.monotonic()
    while True:
        orientation = sense.get_orientation_radians()
        # Order is yaw, pitch, roll. The Go side must parse it in this order.
        print([orientation["yaw"], orientation["pitch"], orientation["roll"]], flush=True)

        next_t += POLL_SECONDS
        delay = next_t - time.monotonic()
        if delay > 0:
            time.sleep(delay)
        else:
            next_t = time.monotonic()  # fell behind; don't try to catch up


if __name__ == "__main__":
    main()
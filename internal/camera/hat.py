from sense_hat import SenseHat
import time

sense = SenseHat()

# IMU fusion inputs: (compass, gyroscope, accelerometer). Default is all three.
# sense.set_imu_config(False, True, True)  # compass off: yaw drifts instead of wobbling

# Talk to RTIMULib directly so we read each real IMU sample exactly once.
#  - The original loop re-read the same sample thousands of times per second
#    (duplicates flooding the Go side).
#  - A fixed sleep (my previous version) can fall out of step with the IMU's own
#    sample rate; RTIMULib is meant to be polled at its poll interval.
imu = sense._imu
POLL = max(imu.IMUGetPollInterval() / 1000.0, 0.002)  # seconds


def main():
    while True:
        if imu.IMURead():
            d = imu.getIMUData()
            if d.get("fusionPoseValid"):
                roll, pitch, yaw = d["fusionPose"]  # RTIMULib order: roll, pitch, yaw
                # Output order is yaw, pitch, roll (unchanged, matches the Go parser's input)
                print([yaw, pitch, roll], flush=True)
        time.sleep(POLL / 2)


if __name__ == "__main__":
    main()
"""Diagnostic for the Sense HAT. Run on the Pi:  python3 hat_debug.py

Prints ~4 lines/second:
  rate   - real IMU samples per second (new data, not duplicates)
  r/p/y  - fused roll, pitch, yaw in DEGREES (RTIMULib order roll, pitch, yaw)
  q      - fused quaternion exactly as RTIMULib returns it
  |mag|  - magnetometer field strength in microtesla
  JUMP   - printed whenever yaw/pitch/roll changes by > 4 degrees between two
           consecutive samples (i.e. a glitch in the SENSOR stream itself)

Tests to run (write down what you see):
  A. Lay it flat and still for 10 s.            -> do r/p/y/|mag| stay put? any JUMP lines?
  B. Flat, rotate clockwise (seen from above)
     90 degrees at a time, N -> E -> S -> W.     -> which way does yaw move, and by how much?
  C. Keep rotating slowly in place, flat.        -> |mag| should stay roughly constant
                                                    (Earth's field is ~25-65 uT). If it swings
                                                    a lot, or is far outside that range, the
                                                    magnetometer is uncalibrated / disturbed.
  D. Hold it the way you hold it when using the
     app, pointing at a landmark you know the
     compass bearing of.                         -> note r/p/y and the true bearing.
"""
import math
import time

from sense_hat import SenseHat

sense = SenseHat()
imu = sense._imu
poll = max(imu.IMUGetPollInterval() / 1000.0, 0.002)
print("IMU poll interval: %.1f ms" % (poll * 1000))

n = 0
rate_t0 = time.monotonic()
rate = 0.0
last_print = 0.0
prev = None

while True:
    if imu.IMURead():
        d = imu.getIMUData()
        n += 1
        now = time.monotonic()

        if now - rate_t0 >= 1.0:
            rate = n / (now - rate_t0)
            n = 0
            rate_t0 = now

        if d.get("fusionPoseValid"):
            roll, pitch, yaw = [math.degrees(v) for v in d["fusionPose"]]
            cur = (roll, pitch, yaw)

            if prev is not None:
                for name, a, b in zip(("roll", "pitch", "yaw"), prev, cur):
                    diff = (b - a + 180.0) % 360.0 - 180.0
                    if abs(diff) > 4.0:
                        print("JUMP %-5s %+7.2f deg in one sample (t=%.2f)" % (name, diff, now))
            prev = cur

            if now - last_print >= 0.25:
                last_print = now
                q = d.get("fusionQPose")
                mag = d.get("compass")
                magn = math.sqrt(sum(c * c for c in mag)) if mag else float("nan")
                qs = " ".join("%+.3f" % c for c in q) if q else "n/a"
                print("rate %5.1f Hz | r %+7.1f p %+7.1f y %+7.1f | q(%s) | |mag| %5.1f uT"
                      % (rate, roll, pitch, yaw, qs, magn))

    time.sleep(poll / 2)
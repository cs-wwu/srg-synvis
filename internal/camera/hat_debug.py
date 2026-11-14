"""Diagnostic for the Sense HAT.  Run on the Pi, with the app CLOSED:
    python3 internal/camera/hat_debug.py

First make sure no other reader is holding the IMU:
    pgrep -af hat.py        (kill strays with: pkill -f hat.py)

Every second it prints a [status] line, even if no data arrives:
    true/false - how many IMURead() calls returned new data / no data
    valid      - whether the fused pose was valid on the last sample
Data lines (~4/s) show fused roll/pitch/yaw in DEGREES, the raw quaternion,
and the magnetometer field strength. JUMP lines flag a >4 degree change
between two consecutive samples (a glitch in the sensor stream itself).

If no data arrives for 2 s it falls back to sense.get_orientation_radians()
so we can tell whether the library works at all.

Tests (write down what you see):
  A. Flat and still for 10 s            -> values steady? any JUMP lines?
  B. Flat, rotate clockwise N->E->S->W   -> which way does yaw move, by how much?
  C. Rotate slowly in place, flat        -> |mag| should stay ~constant (25-65 uT)
  D. Hold it the way you use the app, pointed at a known bearing -> note r/p/y
"""
import math
import time

from sense_hat import SenseHat

sense = SenseHat()
imu = sense._imu
poll = max(imu.IMUGetPollInterval() / 1000.0, 0.002)
print("IMU poll interval: %.1f ms" % (poll * 1000), flush=True)

t_start = time.monotonic()
t_status = t_start
t_data = 0.0
rate_t0 = t_start
n_true = n_false = 0   # since last status line
n_rate = 0
rate = 0.0
valid = None
prev = None
fallback_t = 0.0

while True:
    now = time.monotonic()

    if imu.IMURead():
        n_true += 1
        n_rate += 1
        d = imu.getIMUData()
        valid = bool(d.get("fusionPoseValid"))

        if now - rate_t0 >= 1.0:
            rate = n_rate / (now - rate_t0)
            n_rate = 0
            rate_t0 = now

        if valid:
            roll, pitch, yaw = [math.degrees(v) for v in d["fusionPose"]]
            cur = (roll, pitch, yaw)
            if prev is not None:
                for name, a, b in zip(("roll", "pitch", "yaw"), prev, cur):
                    diff = (b - a + 180.0) % 360.0 - 180.0
                    if abs(diff) > 4.0:
                        print("JUMP %-5s %+7.2f deg in one sample (t=%.2f)" % (name, diff, now - t_start), flush=True)
            prev = cur

            if now - t_data >= 0.25:
                t_data = now
                q = d.get("fusionQPose")
                mag = d.get("compass")
                magn = math.sqrt(sum(c * c for c in mag)) if mag else float("nan")
                qs = " ".join("%+.3f" % c for c in q) if q else "n/a"
                print("rate %5.1f Hz | r %+7.1f p %+7.1f y %+7.1f | q(%s) | |mag| %5.1f uT"
                      % (rate, roll, pitch, yaw, qs, magn), flush=True)
    else:
        n_false += 1

    if now - t_status >= 1.0:
        print("[status] t=%.0fs IMURead true=%d false=%d valid=%s"
              % (now - t_start, n_true, n_false, valid), flush=True)
        n_true = n_false = 0
        t_status = now

        # Nothing usable for 2+ seconds: does the plain library call work?
        if now - t_data > 2.0 and now - t_start > 2.0:
            o = sense.get_orientation_radians()
            print("[fallback] sense.get_orientation_radians(): yaw %+.1f pitch %+.1f roll %+.1f deg"
                  % (math.degrees(o["yaw"]), math.degrees(o["pitch"]), math.degrees(o["roll"])), flush=True)

    time.sleep(poll / 2)
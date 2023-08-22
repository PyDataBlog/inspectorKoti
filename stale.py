import time

# Consume CPU for n seconds
start_time = time.time()
while time.time() - start_time < 30:
    pass

# Go idle
while True:
    time.sleep(10)
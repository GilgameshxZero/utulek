# Utlities for timing.

time_result = {}


def time(key, function, *args, **kwargs):
    import time

    global time_result

    if key not in time_result:
        time_result[key] = 0

    exec_begin = time.process_time()
    result = function(*args, **kwargs)
    time_result[key] += time.process_time() - exec_begin

    return result

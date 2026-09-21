from utils import helper as h, scale
import os
import os.path


def run():
    a = h(1)
    b = scale(a)
    c = os.getenv("HOME")
    return os.path.join(a, b, c)
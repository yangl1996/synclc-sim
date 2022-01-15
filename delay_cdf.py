#!/usr/bin/python

import math
import os
import sys
import signal
from argparse import ArgumentParser
from time import sleep, time
import glob
import re
import json
from dateutil.parser import *
import numpy

parser = ArgumentParser(description="SyncLC data processing")
parser.add_argument('--dir',
                    type=str,
                    help="Directory of the result files",
                    default=".")

# Expt parameters
args = parser.parse_args()

datapoints = []

if __name__ == "__main__":
    files=glob.glob(args.dir+"/*-delay.txt")
    for filepath in files:
        with open(filepath) as f:
            for line in f:
                stripped = line.strip()
                if stripped != "":
                    datapoints.append(float(stripped)/1000.0)

N = len(datapoints)
x = numpy.sort(datapoints)
y = numpy.array(range(N))/float(N)

print("midpoint", "density")
for i in range(N):
    print(x[i], y[i])


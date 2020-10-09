#!/usr/bin/env python3

import os, sys

files = sys.argv[1:-1]
keep = int(sys.argv[-1])

files = sorted(files, key=os.path.getmtime)

diff = len(files) - keep

print('Keeping {} most recent files.'.format(keep))
print('Got {} files.'.format(len(files)))

for x in range (0, diff):
    print('\tDeleting {}.'.format(files[x]))
    os.remove(files[x])
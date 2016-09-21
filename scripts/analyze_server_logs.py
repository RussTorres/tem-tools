import sys
import re


def usage():
    print(sys.argv[0] + ' <filename> <serverlog> ')


def run():
    if len(sys.argv) < 3:
        usage()
        return
    fn = sys.argv[1]
    serverlogfn = sys.argv[2]

    f = open(fn)

    for line in iter(f):
        m = re.match('(\d{4}/\d{2}/\d{2} \d{2}:\d{2}):(\d{2}(\.\d*)?),(.+\.tif),(\d+),(\d+)', line)
        if m:
            timestamp = m.group(1)
            tile_filename = m.group(4)
            acq_separator_index = tile_filename.find(':')
            if acq_separator_index >= 0:
                tile_filename = tile_filename[acq_separator_index + 1:]
            print('grep "%s.*%s" %s' % (timestamp, tile_filename, serverlogfn))

    f.close()

if __name__ == '__main__':
    run()

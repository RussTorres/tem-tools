import sys
import math
import re


def usage():
    print(sys.argv[0] + ' <filename> <tilesize {5m | 20m}> [<output file>]')


def run():
    if len(sys.argv) < 3:
        usage()
        return
    fn = sys.argv[1]
    if sys.argv[2] == '5m':
        tilesize_param = '5557452'
    elif sys.argv[2] == '20m':
        tilesize_param = '20971520'
    else:
        print('Invalid tile size - valid values are: {5m, 20m} ')
        usage()
        return
    outputfilename = None
    if len(sys.argv) > 3:
        outputfilename = sys.argv[3]

    print(fn, tilesize_param)

    f = open(fn)
    tilesize_substr = '(' + tilesize_param + ') in '

    of = None
    if outputfilename is not None:
        of = open(outputfilename, 'w')

    count_le_timelimit = 0
    count_gt_timelimit = 0
    total_count = 0
    min_value = 0
    max_value = 0
    m1stat = 0
    m2stat = 0
    min_time_left = 0
    max_time_left = 0

    for line in iter(f):
        tilesize_substr_index = line.find(tilesize_substr)
        timeleft_index = line.find('time left ')
        if tilesize_substr_index < 0 or timeleft_index < 0:
            continue

        total_time = 0
        total_time_end_index = line.find('ms', tilesize_substr_index, timeleft_index)
        if total_time_end_index >= 0:
            total_time = math.ceil(float(line[tilesize_substr_index + len(tilesize_substr):total_time_end_index]))
        else:
            total_time_end_index = line.find('s', tilesize_substr_index, timeleft_index)
            if total_time_end_index >= 0:
                total_time = math.ceil(float(line[tilesize_substr_index + len(tilesize_substr):total_time_end_index]) * 1000)
            else:
                print("Could not extract the total time from %s" % line)
                continue

        timeleft_end_index = line.find(' ms', timeleft_index)
        time_left = -int(line[timeleft_index + len('time left '):timeleft_end_index])

        if time_left > 0:
            m = re.match('(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(\.\d*)?) Sent (.+\.tif)', line)
            if m:
                request_timestamp = m.group(1)
                tile_filename = m.group(3)
                print("%s,%s,%s,%d,%d" % (request_timestamp, tile_filename, tilesize_param, total_time, time_left), file=of)

        if total_count == 0:
            total_count = 1
            min_value = total_time
            max_value = total_time
            m1stat = total_time
            m2stat = 0
            min_time_left = time_left
            max_time_left = time_left
        else:
            total_count = total_count + 1
            delta = total_time - m1stat
            delta1 = delta / total_count
            m1stat = m1stat + delta1
            m2stat = m2stat + delta1 * delta * (total_count - 1)
            min_value = min(total_time, min_value)
            max_value = max(total_time, max_value)
            min_time_left = min(time_left, min_time_left)
            max_time_left = max(time_left, max_time_left)

        if time_left <= 0:
            count_le_timelimit = count_le_timelimit + 1
        else:
            count_gt_timelimit = count_gt_timelimit + 1

    f.close()
    if of is not None:
        of.close()

    print('Total count: %d' % total_count)
    if total_count > 0:
        print('Count below timelimit: %d (%f %%)' % (count_le_timelimit, count_le_timelimit * 100 / total_count))
        print('Count above timelimit: %d (%f %%)' % (count_gt_timelimit, count_gt_timelimit * 100 / total_count))
        print('Avg time: %f ms' % m1stat)
        print('Std time: %f' % ((math.sqrt(m2stat) / (total_count - 1)) if total_count > 1 else 0))
        print('Min time: %d ms' % min_value)
        print('Max time: %d ms' % max_value)
        print('Min time diff below timelimit: %d ms' % min_time_left)
        print('Max time diff above timelimit: %d ms' % max_time_left)


if __name__ == '__main__':
    run()

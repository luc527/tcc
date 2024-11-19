def parse_cpu_data(path):
    data = {}
    with open(path, 'r') as f:
        f.readline() # ignore fst line (header)
        for line in f:
            line = line.rstrip()
            cols = line.split(',')
            timestamp = int(cols[0])
            data[timestamp] = {
                'user': float(cols[1]),
                'system': float(cols[2]),
            }
    return data

def parse_mem_data(path):
    data = {}
    with open(path, 'r') as f:
        f.readline() # ignore fst line (header)
        for line in f:
            line = line.rstrip()
            cols = line.split(',')
            timestamp = int(cols[0])
            data[timestamp] = {
                'uss': int(cols[1]),
            }
    return data

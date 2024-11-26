import sys
import pandas as pd
from common import parse_throughput_pub_line
from collections import defaultdict

try:
    lang = sys.argv[1]
    date = sys.argv[2]
except:
    lang = 'go'
    date = '23nov1045'


path = f'data/{lang}_cli_throughput_{date}.txt'

mps = defaultdict(lambda: defaultdict(lambda: defaultdict(int)))

with open(path, 'r') as f:
    for line in f:
        line = line.rstrip()
        if (obj := parse_throughput_pub_line(line)):
            mps[obj['topic']][obj['publisher']][obj['timestamp']] += 1

# actual number of messages each publisher was able to send in each second

# although the right place to put the log would be after tconn.publish()
# ... redo these tests once more?

mps_de = [{'topic': topic,
           'publisher': publisher,
           'timestamp': timestamp,
           'count': mps[topic][publisher][timestamp]}
          for topic in mps
          for publisher in mps[topic]
          for timestamp in mps[topic][publisher]]

mps_df = pd.DataFrame(mps_de)

for k in [3, 5, 10]:
    mps_df[f'timestamp{k}'] = k * (mps_df['timestamp'] // k)

# messages per each second:   mps_df.groupby('timestamp')['count'].sum()
# mps in frames of 3 seconds: mps_df.groupby('timestamp3')['count'].sum() / 3
# mps in frames of 5 seconds: mps_df.groupby('timestamp5')['count'].sum() / 5, etc.

# graph can just be
# y axis: # messages sent at this second
# x axis: seconds elapsed



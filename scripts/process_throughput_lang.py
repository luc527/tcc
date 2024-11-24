import sys, os
from pprint import pprint
from common import parse_cpu_data, parse_mem_data, parse_throughput_data, reindex, to_y, to_df
import pandas as pd
import matplotlib.pyplot as plt

try:
    lang = sys.argv[1]
    date = sys.argv[2]
except IndexError:
    print('usage: <lang> <date>\n')
    exit()

if not os.path.exists('./data'):
    print('run this scipt at the root of the repository\n')
    exit()

test = 'throughput'
cli_ext = 'txt'

cpu_path = f'data/{lang}_cpu_{test}_{date}.csv'
mem_path = f'data/{lang}_mem_{test}_{date}.csv'
cli_path = f'data/{lang}_cli_{test}_{date}.{cli_ext}'

paths         = [cpu_path, mem_path, cli_path]
missing_paths = [path for path in paths if not os.path.exists(path)]
if missing_paths:
    for path in missing_paths:
        print(f'{path} not present')
    exit()

tru_data, act_data = parse_throughput_data(cli_path)
beginning = min(tru_data.keys())
ending    = max(tru_data.keys())
tru_data  = reindex(tru_data, beginning, ending)
act_data  = reindex(act_data, beginning, ending)
cpu_df = to_df(reindex(parse_cpu_data(cpu_path), beginning, ending))
mem_df = to_df(reindex(parse_mem_data(mem_path), beginning, ending))

tru_data_degrouped = [{'timestamp': t, **v}
                      for t, vs in tru_data.items()
                      for v in vs]

tru_df = pd.DataFrame(tru_data_degrouped)
tru_df.loc[tru_df['throughput_psec'] >= 1000, 'throughput_psec'] = float('nan')
tru_df.bfill(inplace=True)

cpu_df = cpu_df.rolling(5).mean().bfill()

x = range(tru_df.timestamp.min(), tru_df.timestamp.max()+1)

cpu_df = cpu_df.reindex(x, method='nearest')
mem_df = mem_df.reindex(x, method='nearest')

plt.xlabel('Segundos após início do teste')

plt.grid(visible=True, axis='y')

# plt.ylabel('% CPU')
# cpu_usr_y = [cpu_df['user'][t] for t in x]
# cpu_sys_y = [cpu_df['system'][t] for t in x]
# plt.plot(x, cpu_usr_y, 'b--', cpu_sys_y, 'b:', linewidth=1)
# plt.legend(['Usuário', 'Sistema'])
# max_y = max(*cpu_usr_y, *cpu_sys_y)

plt.ylabel('Memória (mb)')
mem_y = [mem_df['uss'][t] / 1024 / 1024 for t in x]
plt.plot(x, mem_y)
max_y = max(mem_y)

plt.xticks(list(act_data.keys()))
for x, o in act_data.items():
    plt.axvline(x, color='grey', linewidth=1)
    plt.text(x+1, 1/7 * max_y, f'{o['topics_per_conn']} i/c', rotation=0)

plt.show()
import os
import psutil

p = psutil.Process(os.getpid())
print(p.cpu_num())
print(p.memory_full_info())
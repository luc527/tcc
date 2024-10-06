defmodule Tcc.Clients do
  use GenServer
  alias Tcc.Clients.Partition
  import Tcc.Utils
  require Logger

  def start_link(nparts) do
    GenServer.start_link(__MODULE__, nparts, name: __MODULE__)
  end

  @impl true
  def init(nparts) do
    {:ok, nparts, {:continue, :start_partitions}}
  end

  @impl true
  def handle_continue(:start_partitions, nparts) do
    (0..nparts-1) |> Enum.each(&Partition.start/1)
    {:noreply, nparts}
  end

  @impl true
  def handle_call(:num_partitions, _from, nparts) do
    {:reply, nparts, nparts}
  end

  @impl true
  def handle_call({:register, cid_or_nil}, {pid, _tag}, nparts) do
    cid = cid_or_nil || unique_integer()
    partid = rem(cid, nparts)
    {:reply, Partition.register(partid, cid, pid), nparts}
  end

  defp num_partitions() do
    GenServer.call(__MODULE__, :num_partitions)
  end

  def which_partition(client) do
    rem(client, num_partitions())
  end

  def register(cid_or_nil) do
    GenServer.call(__MODULE__, {:register, cid_or_nil})
  end

  def join(room, name, cid) do
    Partition.join(which_partition(cid), room, name, cid)
  end

  def exit(room, cid) do
    Partition.exit(which_partition(cid), room, cid)
  end

  def talk(room, text, cid) do
    Partition.talk(which_partition(cid), room, text, cid)
  end

  def list_rooms(cid) do
    Partition.list_rooms(which_partition(cid), cid)
  end

  def send(cid, msg) do
    Partition.send(which_partition(cid), cid, msg)
  end

  def send_batch(cids, msg) do
    n = num_partitions()
    Enum.group_by(cids, &rem(&1, n))
    |> Enum.each(fn {partid, cids} -> Partition.send_batch(partid, cids, msg) end)
  end
end

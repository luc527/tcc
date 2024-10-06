defmodule Tcc.Rooms do
  require Logger
  alias Tcc.Rooms
  use GenServer

  def start_link(nparts) do
    GenServer.start_link(__MODULE__, nparts, name: __MODULE__)
  end

  @impl true
  def init(nparts) do
    {:ok, nparts, {:continue, :start_partitions}}
  end

  @impl true
  def handle_continue(:start_partitions, nparts) do
    0..(nparts - 1) |> Enum.each(&Rooms.Partition.start/1)
    {:noreply, nparts}
  end

  @impl true
  def handle_call(:num_partitions, _from, nparts) do
    {:reply, nparts, nparts}
  end

  defp num_partitions() do
    GenServer.call(__MODULE__, :num_partitions)
  end

  defp which_partition(room) do
    rem(room, num_partitions())
  end

  def join(room, name, client) do
    Rooms.Partition.join(which_partition(room), room, name, client)
  end

  def exit(room, name, client) do
    Rooms.Partition.exit(which_partition(room), room, name, client)
  end

  def talk(room, name, text) do
    Rooms.Partition.talk(which_partition(room), room, name, text)
  end

  def exit_all_async(rooms_names, cid) do
    nparts = num_partitions()

    rooms_names
    |> Enum.each(fn {room, name} ->
      partid = rem(room, nparts)
      Rooms.Partition.exit_async(partid, room, name, cid)
    end)
  end
end

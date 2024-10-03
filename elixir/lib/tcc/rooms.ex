defmodule Tcc.Rooms do
  require Logger
  use GenServer

  def start_link(npart) do
    GenServer.start_link(__MODULE__, npart, name: __MODULE__)
  end

  @impl true
  def init(npart) do
    Logger.info("rooms: starting")
    {:ok, npart, {:continue, :start_partitions}}
  end

  @impl true
  def handle_continue(:start_partitions, npart) do
    0..(npart - 1) |> Enum.each(&start_partition/1)
    {:noreply, npart}
  end

  @impl true
  def handle_call({:map_partitions, rooms}, _from, npart) do
    result = rooms |> Enum.map(&rem(&1, npart))
    {:reply, result, npart}
  end

  defp start_partition(id) do
    Logger.info("rooms: starting partition #{id}")
    tables = %{
      room_client: :ets.new(nil, [:ordered_set, :public]),
      room_name: :ets.new(nil, [:ordered_set, :public])
    }

    DynamicSupervisor.start_child(Tcc.Rooms.Partition.Supervisor, {Tcc.Rooms.Partition, {id, tables}})
  end

  def map_partitions(rooms) do
    GenServer.call(__MODULE__, {:map_partitions, rooms})
  end

  # TODO: bottleneck?
  defp which_partition(room) do
    hd(map_partitions([room]))
  end

  def join(room, name, client) do
    Tcc.Rooms.Partition.join(which_partition(room), room, name, client)
  end

  def exit(room, name, client) do
    Tcc.Rooms.Partition.exit(which_partition(room), room, name, client)
  end

  def talk(room, name, text) do
    Tcc.Rooms.Partition.talk(which_partition(room), room, name, text)
  end
end

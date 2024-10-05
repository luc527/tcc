defmodule Tcc.Rooms do
  require Logger
  alias Tcc.Rooms
  use GenServer

  def start_link(nparts) do
    GenServer.start_link(__MODULE__, nparts, name: __MODULE__)
  end

  @impl true
  def init(nparts) do
    Logger.info("rooms: starting")
    {:ok, nparts, {:continue, :start_partitions}}
  end

  @impl true
  def handle_continue(:start_partitions, nparts) do
    0..(nparts - 1) |> Enum.each(&start_partition/1)
    {:noreply, nparts}
  end

  @impl true
  def handle_call(:num_partitions, _from, nparts) do
    {:reply, nparts, nparts}
  end

  defp start_partition(id) do
    Logger.info("rooms: starting partition #{id}")
    tables = %{
      room_client: :ets.new(nil, [:ordered_set, :public]),
      room_name: :ets.new(nil, [:ordered_set, :public])
    }

    DynamicSupervisor.start_child(Rooms.Partition.Supervisor, {Rooms.Partition, {id, tables}})
  end

  def num_partitions() do
    GenServer.call(__MODULE__, :num_partitions)
  end

  def join(part_id, room, name, client) do
    Rooms.Partition.join(part_id, room, name, client)
  end

  def exit(part_id, room, name, client) do
    Rooms.Partition.exit(part_id, room, name, client)
  end

  def talk(part_id, room, name, text) do
    Rooms.Partition.talk(part_id, room, name, text)
  end
end

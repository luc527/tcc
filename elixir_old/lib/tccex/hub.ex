defmodule Tccex.Hub do
  require Logger
  alias Tccex.{Hub, Client}
  use GenServer

  def start_link(partitions: n) do
    GenServer.start_link(__MODULE__, n, name: __MODULE__)
  end

  defp start_tables() do
    [
      room_client_table: :ets.new(nil, [:ordered_set, :public]),
      room_name_table: :ets.new(nil, [:ordered_set, :public]),
      client_rooms_table: :ets.new(nil, [:bag, :public])
    ]
  end

  defp start_partitions(0), do: :ok
  defp start_partitions(n) do
    id = n-1
    tables = start_tables() # TODO: different set of tables per partition... does this actually help?
    {:ok, _} = DynamicSupervisor.start_child(Hub.Partition.Supervisor, {Hub.Partition, {id, tables}})
    start_partitions(n-1)
  end

  @impl true
  def init(n) do
    Logger.info("hub: starting with #{n} partitions")
    {:ok, %{n: n, clis: %{}}, {:continue, :start}}
  end

  @impl true
  def handle_continue(:start, %{n: n}=state) do
    start_partitions(n)
    {:noreply, state}
  end

  # TODO: bottleneck?
  @impl true
  def handle_call({:id, room}, _from, %{n: n}=state) do
    {:reply, rem(room, n), state}
  end

  @impl true
  def handle_call(:partitions, _from, %{n: n}=state) do
    {:reply, n, state}
  end

  # TODO: when unregistered, might be transient, so start timer to exit for real in 1sec
  # if registered again within that 1sec, cancel the timer

  @impl true
  def handle_info({:unregister, Client.Registry, client_id, _}, state) do
    Logger.info("hub: client #{inspect(client_id)} unregistered (?)")
    {:noreply, state}
  end

  @impl true
  def handle_info({:register, Client.Registry, client_id, pid, _}, state) do
    Logger.info("hub: pid #{inspect(pid)} registered under #{inspect(client_id)}")
    Process.monitor(pid)
    state = put_in(state.clis[pid], client_id)
    {:noreply, state}
  end

  @impl true
  def handle_info({:DOWN, _, :process, pid, _}, n) do
    Logger.info("hub: pid #{inspect(pid)} is down")
    {:noreply, n}
  end

  def id(room) do
    GenServer.call(__MODULE__, {:id, room})
  end

  def num_partitions() do
    GenServer.call(__MODULE__, :partitions)
  end

  def join(room, name, client) do
    Hub.Partition.join(id(room), room, name, client)
  end

  def exit(room, client) do
    Hub.Partition.exit(id(room), room, client)
  end

  def talk(room, text, client) do
    Hub.Partition.talk(id(room), room, text, client)
  end

  def lsro(client) do
    0..num_partitions()-1
    |> Task.async_stream(&Hub.Partition.lsro(&1, client))
    |> Enum.flat_map(fn
      {:ok, v} -> [v]
      _ -> []
    end)
    |> Enum.reduce(&Enum.concat/2)
  end
end

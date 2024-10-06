defmodule Tcc.Rooms.Partition do
  use GenServer
  import Tcc.Utils
  alias Tcc.Rooms.Tables
  alias Tcc.Rooms.Partition
  alias Tcc.Clients

  @max_clients 256

  def start(id) do
    ts = Tables.new()
    spec = {Partition, {id, ts}}

    with {:ok, _} <- DynamicSupervisor.start_child(Partition.Supervisor, spec) do
      {:ok, id}
    end
  end

  def start_link({id, ts}) do
    GenServer.start_link(__MODULE__, {id, ts}, name: via(id))
  end

  defp via(id) do
    {:via, Registry, {Partition.Registry, id}}
  end

  defp do_join(room, name, client, ts) do
    with :ok <- check(not Tables.has_client?(ts, room, client), :joined),
         :ok <- check(Tables.client_count(ts, room) < @max_clients, :room_full),
         :ok <- check(not Tables.has_name?(ts, room, name), :name_in_use) do
      Tables.insert(ts, room, name, client)
      {:ok, {:jned, room, name}}
    end
  end

  defp do_exit(room, name, client, ts) do
    with :ok <- check(Tables.has_name?(ts, room, name), :bad_room),
         :ok <- check(Tables.has_client?(ts, room, client), :bad_room) do
      Tables.delete(ts, room, name, client)
      {:ok, {:exed, room, name}}
    end
  end

  defp do_talk(room, name, text, ts) do
    with :ok <- check(Tables.has_name?(ts, room, name), :bad_room) do
      {:ok, {:hear, room, name, text}}
    end
  end

  @impl true
  def init({id, ts}) do
    state = %{
      id: id,
      tables: ts,
    }

    {:ok, state}
  end

  @impl true
  def handle_call({:join, room, name, client}, _from, %{tables: ts}=state) do
    do_join(room, name, client, ts)
    |> handle_result(room, client, state)
  end

  @impl true
  def handle_call({:exit, room, name, client}, _from, %{tables: ts}=state) do
    do_exit(room, name, client, ts)
    |> handle_result(room, client, state)
  end

  @impl true
  def handle_call({:talk, room, name, text, client}, _from, %{tables: ts}=state) do
    do_talk(room, name, text, ts)
    |> handle_result(room, client, state)
  end

  @impl true
  def handle_cast({:exit, room, name, client}, %{tables: ts}=state) do
    do_exit(room, name, client, ts)
    |> handle_result_async(room, state)
  end

  @impl true
  def handle_continue({:broadcast, room, msg}, %{tables: ts}=state) do
    clients = Tables.clients(ts, room)
    Clients.send_batch(clients, msg)
    {:noreply, state}
  end

  @impl true
  def handle_continue({:send_to, client, msg}, state) do
    Clients.send(client, msg)
    {:noreply, state}
  end

  defp handle_result({:ok, msg}, _client, room, state) do
    {:reply, :ack, state, {:continue, {:broadcast, room, msg}}}
  end

  defp handle_result({:error, e}, client, _room, state) do
    {:reply, :ack, state, {:continue, {:send_to, client, {:prob, e}}}}
  end

  defp handle_result_async({:ok, msg}, room, state) do
    {:noreply, state, {:continue, {:broadcast, room, msg}}}
  end

  defp handle_result_async(_error, _room, state) do
    {:noreply, state}
  end

  def join(id, room, name, client) do
    GenServer.call(via(id), {:join, room, name, client})
  end

  def exit(id, room, name, client) do
    GenServer.call(via(id), {:exit, room, name, client})
  end

  def exit_async(id, room, name, client) do
    GenServer.cast(via(id), {:exit, room, name, client})
  end

  def talk(id, room, name, text) do
    GenServer.call(via(id), {:talk, room, name, text})
  end
end

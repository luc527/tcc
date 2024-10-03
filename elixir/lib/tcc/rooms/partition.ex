defmodule Tcc.Rooms.Partition do
  require Logger
  use GenServer
  import Tcc.Utils

  @max_clients 256
  @max_name_length 24
  @max_message_length 2048

  def start_link({id, tables}) do
    GenServer.start_link(__MODULE__, {id, tables}, name: via(id))
  end

  defp via(id) do
    {:via, Registry, {Tcc.Rooms.Partition.Registry, id}}
  end

  defp do_join(room, name, client, tables) do
    with :ok <- check(not has_name?(room, name, tables), :name_in_use),
         :ok <- check(not has_client?(room, client, tables), :joined),
         :ok <- check(valid_name?(name), :bad_name),
         :ok <- check(client_count(room, tables) < @max_clients, :room_full) do
      insert(room, name, client, tables)
      {:ok, {:jned, room, name}}
    end
  end

  defp do_exit(room, name, client, tables) do
    with :ok <- check(has_name?(room, name, tables), :bad_room),
         :ok <- check(has_client?(room, client, tables), :bad_room) do
      delete(room, name, client, tables)
      {:ok, {:exed, room, name}}
    end
  end

  defp do_talk(room, name, text, tables) do
    with :ok <- check(has_name?(room, name, tables), :bad_room),
         :ok <- check(valid_message_text?(text), :bad_message) do
      {:ok, {:hear, room, name, text}}
    end
  end

  @impl true
  def init({id, tables}) do
    tables = %{
      id: id,
      room_name: tables.room_name,
      room_client: tables.room_client
    }

    {:ok, tables}
  end

  @impl true
  def handle_call({:join, room, name, client}, _from, tables) do
    do_join(room, name, client, tables) |> handle_result(room, tables)
  end

  @impl true
  def handle_call({:exit, room, name, client}, _from, tables) do
    do_exit(room, name, client, tables) |> handle_result(room, tables)
  end

  @impl true
  def handle_call({:talk, room, name, text}, _from, tables) do
    do_talk(room, name, text, tables) |> handle_result(room, tables)
  end

  @impl true
  def handle_cast({:exit, room, name, client}, tables) do
    _ = do_exit(room, name, client, tables)
    {:noreply, tables}
  end

  @impl true
  def handle_continue({:broadcast, room, msg}, tables) do
    broadcast(room, msg, tables)
    {:noreply, tables}
  end

  defp handle_result({:ok, msg}, room, tables) do
    {:reply, :ok, tables, {:continue, {:broadcast, room, msg}}}
  end

  defp handle_result(error, _room, tables) do
    {:reply, error, tables}
  end

  defp has_name?(room, name, %{room_name: table}) do
    [] != :ets.lookup(table, {room, name})
  end

  defp has_client?(room, client, %{room_client: table}) do
    [] != :ets.lookup(table, {room, client})
  end

  defp valid_name?(name) do
    n = byte_size(name)
    n > 0 and n <= @max_name_length and not String.contains?(name, "\n")
  end

  defp valid_message_text?(text) do
    n = byte_size(text)
    n > 0 and n <= @max_message_length
  end

  defp insert(room, name, client, %{room_client: rc, room_name: rn}) do
    true = :ets.insert_new(rc, {{room, client}})
    true = :ets.insert_new(rn, {{room, name}})
  end

  defp delete(room, name, client, %{room_client: rc, room_name: rn}) do
    true = :ets.delete(rc, {room, client})
    true = :ets.delete(rn, {room, name})
  end

  # TODO: test client_count?
  defp client_count(room, %{room_client: table}) do
    match = {{room, :_}}
    :ets.select_count(table, [{match, [], [true]}])
  end

  defp clients(room, %{room_client: table}) do
    :ets.match(table, {{room, :"$1"}})
    |> Enum.map(fn [id] -> id end)
  end

  defp broadcast(room, message, tables) do
    Tcc.Clients.send_batch(clients(room, tables), message)
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

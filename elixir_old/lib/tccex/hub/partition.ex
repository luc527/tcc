defmodule Tccex.Hub.Partition do
  use GenServer
  alias Tccex.{Client, Hub}
  import Tccex.Utils

  @max_clients 256
  @max_rooms 256

  defstruct [
    :id,
    :room_name_table,
    :room_client_table,
    :client_rooms_table
  ]

  defp via(id) do
    {:via, Registry, {Tccex.Hub.Partition.Registry, id}}
  end

  def start_link({id, tables}) do
    GenServer.start_link(__MODULE__, [{:id, id} | tables], name: via(id))
  end

  def new(fields) do
    struct!(__MODULE__, fields)
  end

  defp insert_room_name(room, name, %{room_name_table: table}) do
    :ets.insert_new(table, {{room, name}})
  end

  defp delete_room_name(room, name, %{room_name_table: table}) do
    :ets.delete(table, {room, name})
  end

  defp exists_room_name(room, name, %{room_name_table: table}) do
    :ets.lookup(table, {room, name}) != []
  end

  defp insert_room_client(room, client, %{room_client_table: table}) do
    :ets.insert_new(table, {{room, client}})
  end

  defp delete_room_client(room, client, %{room_client_table: table}) do
    :ets.delete(table, {room, client})
  end

  defp exists_room_client(room, client, %{room_client_table: table}) do
    :ets.lookup(table, {room, client}) != []
  end

  defp room_client_count(room, %{room_client_table: table}) do
    match = {{room, :_}}
    :ets.select_count(table, [{ match, [], [true] }])
  end

  defp room_clients(room, %{room_client_table: table}) do
    :ets.match(table, {{room, :"$1"}})
    |> Enum.map(fn [id] -> id end)
  end

  defp insert_client_room(client, room, name, %{client_rooms_table: table}) do
    :ets.insert(table, {client, room, name})
  end

  defp delete_client_room(client, room, %{client_rooms_table: table}) do
    :ets.match_delete(table, {client, room, :_})
  end

  defp client_name_in_room(client, room, %{client_rooms_table: table}) do
    case :ets.match(table, {client, room, :"$1"}) do
      [] -> {:error, :bad_room}
      [[name]] -> {:ok, name}
    end
  end

  defp client_room_count(client, %{client_rooms_table: table}) do
    match = {client, :_, :_}
    :ets.select_count(table, [{ match, [], [true] }])
  end

  defp client_rooms(client, %{client_rooms_table: table}) do
    :ets.match(table, {client, :"$1", :"$2"})
    |> Enum.map(fn [room, name] -> {room, name} end)
  end

  defp broadcast(clients, message) do
    Enum.each(clients, fn id -> Client.send_message(id, message) end)
  end

  defp exit_room(room, client, name, tables) do
    delete_room_client(room, client, tables)
    delete_room_name(room, client, tables)
    delete_client_room(room, client, tables)
    broadcast(room_clients(room, tables), {:exed, room, name})
  end

  @impl true
  def init([{:id, id} | _] = fields) do
    Registry.register(Hub.Partition.Registry, id, nil)
    {:ok, new(fields)}
  end

  @impl true
  def handle_call({:join, room, name, client}, _from, tables) do
    with :ok <- check(room_client_count(room, tables) < @max_clients, :room_full),
         :ok <- check(not exists_room_client(room, client, tables), :joined),
         :ok <- check(not exists_room_name(room, name, tables), :name_in_use),
         :ok <- check(client_room_count(client, tables) < @max_rooms, :room_limit) do
      insert_room_client(room, client, tables)
      insert_room_name(room, client, tables)
      insert_client_room(room, client, name, tables)
      broadcast(room_clients(room, tables), {:jned, room, name})
      {:reply, :ok, tables}
    else
      error ->
        {:reply, error, tables}
    end
  end

  @impl true
  def handle_call({:exit, room, client}, _from, tables) do
    with {:ok, name} <- client_name_in_room(room, client, tables) do
      exit_room(room, client, name, tables)
      {:reply, :ok, tables}
    else
      error ->
        {:reply, error, tables}
    end
  end

  @impl true
  def handle_call({:talk, room, text, client}, _from, tables) do
    with {:ok, name} <- client_name_in_room(room, client, tables) do
      broadcast(room_clients(room, tables), {:hear, room, name, text})
      {:reply, :ok, tables}
    else
      error ->
        {:reply, error, tables}
    end
  end

  @impl true
  def handle_call({:lsro, client}, _from, tables) do
    {:reply, client_rooms(client, tables), tables}
  end

  @impl true
  def handle_cast({:exit_all, client}, tables) do
    client_rooms(client, tables)
    |> Enum.each(fn {room, name} -> exit_room(room, client, name, tables) end)
    {:reply, :ok, tables}
  end

  def join(id, room, name, client) do
    GenServer.call(via(id), {:join, room, name, client})
  end

  def exit(id, room, client) do
    GenServer.call(via(id), {:exit, room, client})
  end

  def exit_all(id, client) do
    GenServer.cast(via(id), {:exit_all, client})
  end

  def talk(id, room, text, client) do
    GenServer.call(via(id), {:talk, room, text, client})
  end

  def lsro(id, client) do
    GenServer.call(via(id), {:lsro, client})
  end
end

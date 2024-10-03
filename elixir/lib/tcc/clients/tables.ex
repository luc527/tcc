defmodule Tcc.Clients.Tables do
  @moduledoc """
  The only purpose of this GenServer is to own the tables so that the data isn't lost even when
  Tcc.Clients restarts. And to group functions related to the tables.
  """

  # TODO: maybe partition too? or would it just introduce overhead?

  use GenServer
  alias Tcc.Clients.Tables.{ClientRoomName, ClientProcess, ProcessClient}

  def start_link(arg) do
    GenServer.start_link(__MODULE__, arg, name: __MODULE__)
  end

  @impl true
  def init(_) do
    :ets.new(ClientRoomName, [:named_table, :ordered_set, :public])
    :ets.new(ClientProcess, [:named_table, :set, :public])
    :ets.new(ProcessClient, [:named_table, :set, :public])
    {:ok, nil}
  end

  def register_client(client, pid) do
    # short circuiting is cool
    :ets.insert_new(ClientProcess, {client, pid}) and
      (:ets.insert_new(ProcessClient, {pid, client}) or
         not :ets.delete(ClientProcess, client))
  end

  def unregister_client(client) do
    case client_pid(client) do
      {:ok, pid} ->
        :ets.delete(ClientProcess, client)
        :ets.delete(ProcessClient, pid)
        true

      :not_found ->
        false
    end
  end

  def client_pid(client) do
    case :ets.lookup(ClientProcess, client) do
      [{_, pid}] -> {:ok, pid}
      [] -> :not_found
    end
  end

  def pid_client(pid) do
    case :ets.lookup(ProcessClient, pid) do
      [{_, client}] -> {:ok, client}
      [] -> :not_found
    end
  end

  def clients_pids(clients) do
    spec =
      for client <- clients do
        head = {client, :"$1"}
        {head, [], [:"$1"]}
      end

    for match <- :ets.select(ClientProcess, spec) do
      match
    end
  end

  def enter_room(room, name, client) do
    :ets.insert_new(ClientRoomName, {{client, room}, name})
  end

  def exit_room(room, client) do
    :ets.delete(ClientRoomName, {client, room})
  end

  def name_in_room(room, client) do
    case :ets.lookup(ClientRoomName, {client, room}) do
      [{_, name}] -> {:ok, name}
      [] -> :not_found
    end
  end

  def client_rooms(client) do
    pattern = {{client, :'$1'}, :'$2'}
    :ets.match(ClientRoomName, pattern)
    |> Enum.map(fn [room, name] -> {room, name} end)
  end
end

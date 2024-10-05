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

  def register_client(cid, pid) do
    # short circuiting is cool
    :ets.insert_new(ClientProcess, {cid, pid}) and
      (:ets.insert_new(ProcessClient, {pid, cid}) or
         not :ets.delete(ClientProcess, cid))
  end

  def unregister_client(cid) do
    case client_pid(cid) do
      {:ok, pid} ->
        :ets.delete(ClientProcess, cid)
        :ets.delete(ProcessClient, pid)
        true

      :not_found ->
        false
    end
  end

  def client_pid(cid) do
    case :ets.lookup(ClientProcess, cid) do
      [{_, pid}] -> {:ok, pid}
      [] -> :not_found
    end
  end

  def pid_client(pid) do
    case :ets.lookup(ProcessClient, pid) do
      [{_, cid}] -> {:ok, cid}
      [] -> :not_found
    end
  end

  def clients_pids(cids) do
    spec =
      for cid <- cids do
        head = {cid, :"$1"}
        {head, [], [:"$1"]}
      end

    :ets.select(ClientProcess, spec)
  end

  def enter_room(room, name, cid) do
    true = :ets.insert_new(ClientRoomName, {{cid, room}, name})
  end

  def exit_room(room, cid) do
    true = :ets.delete(ClientRoomName, {cid, room})
  end

  def name_in_room(room, cid) do
    case :ets.lookup(ClientRoomName, {cid, room}) do
      [{_, name}] -> {:ok, name}
      [] -> :not_found
    end
  end

  def client_rooms(cid) do
    pattern = {{cid, :'$1'}, :'$2'}
    :ets.match(ClientRoomName, pattern)
    |> Enum.map(fn [room, name] -> {room, name} end)
  end
end

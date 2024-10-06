defmodule Tcc.Clients.Tables do
  @type t :: %{
          client_room_name: reference,
          client_process: reference,
          process_client: reference
        }

  @spec new() :: t
  def new() do
    %{
      client_room_name: :ets.new(nil, [:ordered_set, :public]),
      client_process: :ets.new(nil, [:set, :public]),
      process_client: :ets.new(nil, [:set, :public])
    }
  end

  @spec register_client(t, cid :: integer, pid) :: boolean
  def register_client(%{client_process: cp, process_client: pc}, cid, pid) do
    # short circuiting is cool
    :ets.insert_new(cp, {cid, pid}) and
      (:ets.insert_new(pc, {pid, cid}) or
         not :ets.delete(cp, cid))
  end

  @spec unregister_client(t, cid :: integer) :: boolean
  def unregister_client(%{client_process: cp, process_client: pc} = ts, cid) do
    case client_pid(ts, cid) do
      {:ok, pid} ->
        :ets.delete(cp, cid)
        :ets.delete(pc, pid)
        true

      :not_found ->
        false
    end
  end

  @spec client_pid(t, cid :: integer) :: {:ok, pid} | :not_found
  def client_pid(%{client_process: cp}, cid) do
    case :ets.lookup(cp, cid) do
      [{_, pid}] -> {:ok, pid}
      [] -> :not_found
    end
  end

  @spec clients_pids(t, cids :: [integer]) :: [pid]
  def clients_pids(%{client_process: cp}, cids) do
    spec =
      for cid <- cids do
        head = {cid, :"$1"}
        {head, [], [:"$1"]}
      end

    :ets.select(cp, spec)
  end

  @spec pid_client(t, pid) :: {:ok, cid :: integer} | :not_found
  def pid_client(%{process_client: pc}, pid) do
    case :ets.lookup(pc, pid) do
      [{_, cid}] -> {:ok, cid}
      [] -> :not_found
    end
  end

  @spec add_room(t, room :: integer, name :: binary, cid :: integer) :: true
  def add_room(%{client_room_name: crn}, room, name, cid) do
    true = :ets.insert_new(crn, {{cid, room}, name})
  end

  @spec remove_room(t, room :: integer, cid :: integer) :: true
  def remove_room(%{client_room_name: crn}, room, cid) do
    true = :ets.delete(crn, {cid, room})
  end

  @spec name_in_room(t, room :: integer, cid :: integer) :: {:ok, name :: binary} | :not_found
  def name_in_room(%{client_room_name: crn}, room, cid) do
    case :ets.lookup(crn, {cid, room}) do
      [{_, name}] -> {:ok, name}
      [] -> :not_found
    end
  end

  @spec client_rooms_names(t, cid :: integer) :: [{room :: integer, name :: binary}]
  def client_rooms_names(%{client_room_name: crn}, cid) do
    pattern = {{cid, :"$1"}, :"$2"}

    :ets.match(crn, pattern)
    |> Enum.map(fn [room, name] -> {room, name} end)
  end

  @spec client_room_count(t, cid :: integer) :: integer
  def client_room_count(%{client_room_name: crn}, cid) do
    head = {{cid, :_}, :_}
    spec = [{head, [], [true]}]
    :ets.select_count(crn, spec)
  end

  @spec client_in_room?(t, cid :: integer, room :: integer) :: bool
  def client_in_room?(%{client_room_name: crn}, cid, room) do
    [] != :ets.lookup(crn, {cid, room})
  end
end

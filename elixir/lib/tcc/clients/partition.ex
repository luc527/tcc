defmodule Tcc.Clients.Partition do
  use GenServer
  require Logger
  import Tcc.Utils
  alias Tcc.Clients.Partition
  alias Tcc.Clients.Tables
  alias Tcc.Rooms

  @max_rooms 256
  @max_name_length 24
  @max_message_length 2048

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

  @impl true
  def init({id, ts}) do
    state = %{
      id: id,
      tables: ts
    }

    {:ok, state}
  end

  @impl true
  def handle_call({:register, cid, pid}, _from, %{tables: ts} = state) do
    result =
      case Tables.pid_client(ts, pid) do
        {:ok, ^cid} ->
          {:ok, cid}

        {:ok, _} ->
          :error

        :not_found ->
          if Tables.register_client(ts, cid, pid) do
            Process.monitor(pid)
            {:ok, cid}
          else
            :error
          end
      end

    {:reply, result, state}
  end

  @impl true
  def handle_call({:join, room, name, cid}, _from, %{tables: ts} = state) do
    with :ok <- check(valid_name?(name), :bad_name),
         :ok <- check(Tables.client_room_count(ts, cid) < @max_rooms, :room_limit),
         :ok <- check(not Tables.client_in_room?(ts, cid, room), :joined),
         :ok <- Rooms.join(room, name, cid) do
      Tables.add_room(ts, room, name, cid)
      {:reply, :ok, state}
    else
      error ->
        {:reply, error, state}
    end
  end

  @impl true
  def handle_call({:exit, room, cid}, _from, %{tables: ts} = state) do
    with {:ok, name} <- Tables.name_in_room(ts, room, cid),
         :ok <- Rooms.exit(room, name, cid) do
      Tables.remove_room(ts, room, cid)
      {:reply, :ok, state}
    else
      :not_found ->
        {:reply, {:error, :bad_room}, state}

      error ->
        {:reply, error, state}
    end
  end

  @impl true
  def handle_call({:talk, room, text, cid}, _from, %{tables: ts} = state) do
    with :ok <- check(valid_message_text?(text), :bad_message),
         {:ok, name} <- Tables.name_in_room(ts, room, cid),
         :ok <- Rooms.talk(room, name, text) do
      {:reply, :ok, state}
    else
      :not_found ->
        {:reply, {:error, :bad_room}, state}

      error ->
        {:reply, error, state}
    end
  end

  @impl true
  def handle_call({:lsro, cid}, _from, %{tables: ts} = state) do
    list = Tables.client_rooms_names(ts, cid)
    msg = {:rols, list}
    # TODO: is this really needed?
    {:reply, :ok, state, {:continue, {:send_to, msg, cid}}}
  end

  @impl true
  def handle_call({:send_batch, cids, msg}, _from, %{tables: ts} = state) do
    Tables.clients_pids(ts, cids)
    |> Enum.each(&send(&1, {:server_msg, msg}))

    {:reply, :ok, state}
  end

  @impl true
  def handle_call({:send_to, msg, cid}, _from, %{tables: ts} = state) do
    send_to(msg, cid, ts)
    {:reply, :ok, state}
  end

  @impl true
  def handle_continue({:send_to, msg, cid}, %{tables: ts} = state) do
    send_to(msg, cid, ts)
    {:noreply, state}
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, pid, _reason}, %{tables: ts} = state) do
    case Tables.pid_client(ts, pid) do
      :not_found ->
        Logger.warning("clients: received #{:erlang.pid_to_list(pid)} down but it's not registered")

        {:noreply, state}

      {:ok, cid} ->
        unregister(cid, ts)
        {:noreply, state}
    end
  end

  defp send_to(msg, cid, ts) do
    with {:ok, pid} <- Tables.client_pid(ts, cid) do
      send(pid, {:server_msg, msg})
    else
      error ->
        Logger.warning("clients: failed to send_to(#{inspect(msg)}, #{cid}), error: #{inspect(error)}")
    end
  end

  defp unregister(cid, ts) do
    Tables.unregister_client(ts, cid)

    Tables.client_rooms_names(ts, cid)
    |> Rooms.exit_all_async(cid)
  end

  defp valid_name?(name) do
    n = byte_size(name)
    n > 0 and n <= @max_name_length and not String.contains?(name, "\n")
  end

  defp valid_message_text?(text) do
    n = byte_size(text)
    n > 0 and n <= @max_message_length
  end

  def join(partid, room, name, cid) do
    GenServer.call(via(partid), {:join, room, name, cid})
  end

  def exit(partid, room, cid) do
    GenServer.call(via(partid), {:exit, room, cid})
  end

  def talk(partid, room, text, cid) do
    GenServer.call(via(partid), {:talk, room, text, cid})
  end

  def list_rooms(partid, cid) do
    GenServer.call(via(partid), {:lsro, cid})
  end

  def register(partid, cid, pid) do
    GenServer.call(via(partid), {:register, cid, pid})
  end

  def send(partid, cid, msg) do
    GenServer.call(via(partid), {:send_to, msg, cid})
  end

  def send_batch(partid, cids, msg) do
    GenServer.call(via(partid), {:send_batch, cids, msg})
  end
end

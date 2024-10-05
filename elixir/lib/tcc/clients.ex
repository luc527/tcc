defmodule Tcc.Clients do
  require Logger
  alias Tcc.{Clients.Tables, Rooms}
  use GenServer

  def start_link(arg) do
    GenServer.start_link(__MODULE__, arg, name: __MODULE__)
  end

  @impl true
  def init(_) do
    {:ok, nil, {:continue, :get_num_partitions}}
  end

  @impl true
  def handle_continue(:get_num_partitions, nil) do
    nparts = Rooms.num_partitions()
    {:noreply, %{nparts: nparts}}
  end

  @impl true
  def handle_continue({:send_to, msg, cid}, state) do
    with {:ok, pid} <- Tables.client_pid(cid) do
      send(pid, {:server_msg, msg})
      {:noreply, state}
    else
      error ->
        Logger.warning("clients: failed to send_to #{inspect(msg)}, #{cid}, error: #{inspect(error)}")
        {:noreply, state}
    end
  end

  @impl true
  def handle_call({:register, cid}, {pid, _tag}, state) do
    {:reply, check_register(cid, pid), state}
  end

  @impl true
  def handle_call({:join, room, name, cid}, _from, %{nparts: nparts} = state) do
    partid = which_partition(room, nparts)

    with :ok <- Rooms.join(partid, room, name, cid) do
      true = Tables.enter_room(room, name, cid)
      {:reply, :ok, state}
    else
      error ->
        {:reply, error, state}
    end
  end

  @impl true
  def handle_call({:exit, room, cid}, _from, %{nparts: nparts} = state) do
    partid = which_partition(room, nparts)

    with {:ok, name} <- Tables.name_in_room(room, cid),
         :ok <- Rooms.exit(partid, room, name, cid) do
      Tables.exit_room(room, cid)
      {:reply, :ok, state}
    else
      :not_found ->
        {:reply, {:error, :bad_room}, state}

      error ->
        {:reply, error, state}
    end
  end

  @impl true
  def handle_call({:talk, room, text, cid}, _from, %{nparts: nparts} = state) do
    partid = which_partition(room, nparts)

    with {:ok, name} <- Tables.name_in_room(room, cid),
         :ok <- Rooms.talk(partid, room, name, text) do
      {:reply, :ok, state}
    else
      :not_found ->
        {:reply, {:error, :bad_room}, state}

      error ->
        {:reply, error, state}
    end
  end

  @impl true
  def handle_call({:lsro, cid}, _from, state) do
    msg = {:rols, Tables.client_rooms(cid)}
    {:reply, :ok, state, {:continue, {:send_to, msg, cid}}}
  end

  @impl true
  def handle_call({:send_batch, clients, msg}, _from, state) do
    Tables.clients_pids(clients)
    |> Enum.each(&send(&1, {:server_msg, msg}))

    {:reply, :ok, state}
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, pid, _reason}, %{nparts: nparts} = state) do
    case Tables.pid_client(pid) do
      :not_found ->
        Logger.warning(
          "clients: received #{:erlang.pid_to_list(pid)} down but it's not registered"
        )

        {:noreply, state}

      {:ok, cid} ->
        Logger.info("clients: unregistering #{:erlang.pid_to_list(pid)} from #{cid}")
        true = Tables.unregister_client(cid)

        Tables.client_rooms(cid)
        |> Enum.each(fn {room, name} ->
          partid = which_partition(room, nparts)
          Rooms.Partition.exit_async(partid, room, name, cid)
        end)

        {:noreply, state}
    end
  end

  defp which_partition(room, nparts), do: rem(room, nparts)

  defp check_register(nil, pid) do
    case Tables.pid_client(pid) do
      {:ok, cid} ->
        {:ok, cid}

      :not_found ->
        cid = :erlang.unique_integer([:monotonic, :positive])
        try_register(cid, pid)
    end
  end

  defp check_register(cid, pid) do
    case Tables.pid_client(pid) do
      {:ok, ^cid} ->
        {:ok, cid}

      {:ok, _} ->
        :error

      :not_found ->
        try_register(cid, pid)
    end
  end

  defp try_register(cid, pid) do
    if Tables.register_client(cid, pid) do
      Process.monitor(pid)
      {:ok, cid}
    else
      :error
    end
  end

  def register(cid) do
    GenServer.call(__MODULE__, {:register, cid})
  end

  def send_batch(clients, message) do
    GenServer.call(__MODULE__, {:send_batch, clients, message})
  end

  def join(room, name, cid) do
    GenServer.call(__MODULE__, {:join, room, name, cid})
  end

  def exit(room, cid) do
    GenServer.call(__MODULE__, {:exit, room, cid})
  end

  def talk(room, text, cid) do
    GenServer.call(__MODULE__, {:talk, room, text, cid})
  end

  def lsro(cid) do
    GenServer.call(__MODULE__, {:lsro, cid})
  end
end

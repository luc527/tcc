defmodule Tcc.Clients do
  require Logger
  alias Tcc.Clients.Tables
  use GenServer

  def start_link(arg) do
    GenServer.start_link(__MODULE__, arg, name: __MODULE__)
  end

  @impl true
  def init(_) do
    {:ok, nil}
  end

  @impl true
  def handle_call({:register, cid}, {pid, _tag}, _) do
    {:reply, check_register(cid, pid), nil}
  end

  @impl true
  def handle_call({:join, room, name, cid}, _from, _) do
    with :ok <- Tcc.Rooms.join(room, name, cid) do
      true = Tables.enter_room(room, name, cid)
      {:reply, :ok, nil}
    else
      error ->
        {:reply, error, nil}
    end
  end

  @impl true
  def handle_call({:exit, room, cid}, _from, _) do
    with {:ok, name} <- Tables.name_in_room(room, cid),
         :ok <- Tcc.Rooms.exit(room, name, cid) do
      Tables.exit_room(room, cid)
      {:reply, :ok, nil}
    else
      :not_found ->
        {:reply, {:error, :bad_room}, nil}

      error ->
        {:reply, error, nil}
    end
  end

  @impl true
  def handle_call({:talk, room, text, cid}, _from, _) do
    with {:ok, name} <- Tables.name_in_room(room, cid),
         :ok <- Tcc.Rooms.talk(room, name, text) do
      {:reply, :ok, nil}
    else
      :not_found ->
        {:reply, {:error, :bad_room}, nil}

      error ->
        {:reply, error, nil}
    end
  end

  # TODO: lsro

  @impl true
  def handle_call({:send_batch, clients, message}, _from, _) do
    Tables.clients_pids(clients)
    |> Enum.each(&send(&1, {:room_msg, message}))

    {:reply, :ok, nil}
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, pid, _reason}, _) do
    case Tables.pid_client(pid) do
      :not_found ->
        Logger.warning(
          "clients: received #{:erlang.pid_to_list(pid)} down but it's not registered"
        )

        {:noreply, nil}

      {:ok, cid} ->
        Logger.info("clients: unregistering #{:erlang.pid_to_list(pid)} from #{cid}")
        true = Tables.unregister_client(cid)

        rooms_names = Tables.client_rooms(cid)

        parts =
          rooms_names
          |> Enum.map(fn {room, _name} -> room end)
          |> Tcc.Rooms.map_partitions()

        rooms_names
        |> Enum.zip(parts)
        |> Enum.each(fn {{room, name}, part} ->
          Tcc.Rooms.Partition.exit_async(part, room, name, cid)
        end)

        {:noreply, nil}
    end
  end

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
end

defmodule Tcc.Client do
  require Logger
  alias Tcc.Clients
  use GenServer, restart: :transient

  def start_link({cid, conn_pid}) do
    GenServer.start_link(__MODULE__, {cid, conn_pid})
  end

  @impl true
  def init({cid, conn_pid}) do
    state = %{client_id: cid, conn_pid: conn_pid}
    Process.monitor(conn_pid)
    {:ok, state, {:continue, :register}}
  end

  @impl true
  def handle_continue(:register, %{client_id: nil}=state) do
    {:ok, cid} = Clients.register(nil)
    state = Map.put(state, :client_id, cid)
    {:noreply, state}
  end

  @impl true
  def handle_continue(:register, %{client_id: cid}=state) do
    {:ok, ^cid} = Clients.register(cid)
    {:noreply, state}
  end

  @impl true
  def handle_info({:server_msg, msg}, %{conn_pid: conn_pid}=state) do
    send(conn_pid, {:server_msg, msg})
    {:noreply, state}
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, pid, _reason}, %{conn_pid: pid}=state) do
    {:stop, :normal, state}
  end

  @impl true
  def handle_call(:id, _from, %{client_id: cid}=state) do
    {:reply, cid, state}
  end

  def send_conn_msg(cid, {:join, room, name}), do: Clients.join(room, name, cid)
  def send_conn_msg(cid, {:exit, room}), do: Clients.exit(room, cid)
  def send_conn_msg(cid, {:talk, room, text}), do: Clients.talk(room, text, cid)
  def send_conn_msg(cid, :lsro), do: Clients.list_rooms(cid)
  def send_conn_msg(_cid, :pong), do: :ok

  def id(pid) do
    GenServer.call(pid, :id)
  end
end

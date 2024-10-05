defmodule Tcc.Client do
  require Logger
  alias Tcc.Clients.Tables
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
  def handle_info({:DOWN, _ref, :process, pid, reason}, %{conn_pid: pid}=state) do
    Logger.info("client stopping, connection ended with reason #{inspect(reason)}")
    {:stop, :normal, state}
  end

  @impl true
  def handle_call({:conn_msg, msg}, _from, %{client_id: cid}=state) do
    {:reply, handle_conn_msg(msg, cid), state}
  end

  @impl true
  def handle_call(:id, _from, %{client_id: cid}=state) do
    {:reply, cid, state}
  end

  defp handle_conn_msg({:join, room, name}, cid) do
    Tcc.Clients.join(room, name, cid)
  end

  defp handle_conn_msg({:exit, room}, cid) do
    Tcc.Clients.exit(room, cid)
  end

  defp handle_conn_msg({:talk, room, text}, cid) do
    Tcc.Clients.talk(room, text, cid)
  end

  defp handle_conn_msg(:lsro, cid) do
    Tcc.Clients.lsro(cid)
  end

  defp handle_conn_msg(:pong, _cid) do
    :ok
  end

  def send_conn_msg(cid, msg) do
    with {:ok, client_pid} <- Tables.client_pid(cid) do
      GenServer.call(client_pid, {:conn_msg, msg})
    end
  end

  def id(pid) do
    GenServer.call(pid, :id)
  end
end

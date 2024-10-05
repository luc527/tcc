defmodule Tccex.Client do
  use GenServer, restart: :transient
  require Logger
  require Tccex.Message
  alias Tccex.{Hub, Message, Client}

  def start_link({client_id, conn_pid}) do
    GenServer.start_link(__MODULE__, {client_id, conn_pid}, name: via(client_id))
  end

  defp via(id) do
    {:via, Registry, {Client.Registry, id}}
  end

  @impl true
  def init({client_id, conn_pid}) do
    state = %{
      client_id: client_id,
      conn_pid: conn_pid
    }
    Process.monitor(conn_pid)
    {:ok, state}
  end

  @impl true
  def handle_cast({:recv, message}, state) do
    handle_recv(message, state)
    {:noreply, state}
  end

  @impl true
  def handle_cast({:send, message}, %{conn_pid: conn_pid} = state) do
    send(conn_pid, {:send, message})
    {:noreply, state}
  end

  @impl true
  def handle_info({:DOWN, _, :process, pid, _}, %{conn_pid: pid}=state) do
    Logger.info("client: connection ended")
    {:stop, :normal, state}
  end

  defp error_to_prob({:error, e}, mtype) do
    if Message.is_message_error(e) do
      {:prob, e}
    else
      {:prob, {:transient, mtype}}
    end
  end

  defp error_to_prob(error, mtype) do
    Logger.warning("client: unhandled error in mtype #{inspect(mtype)}: #{inspect(error)}")
    {:prob, {:transient, mtype}}
  end

  defp handle_recv(:pong, _state) do
    :ok
  end

  defp handle_recv({:join, room, name}, %{client_id: id} = state) do
    Hub.join(room, name, id) |> handle_result(:join, state)
  end

  defp handle_recv({:exit, room}, %{client_id: id} = state) do
    Hub.exit(room, id) |> handle_result(:exit, state)
  end

  defp handle_recv({:talk, room, text}, %{client_id: id} = state) do
    Hub.talk(room, text, id) |> handle_result(:talk, state)
  end

  defp handle_recv(:lsro, %{client_id: id} = state) do
    list = Hub.lsro(id)
    send_to_conn({:rols, list}, state)
  end

  defp handle_result(:ok, _mtype, _state) do
    :ok
  end

  defp handle_result(error, mtype, state) do
    error |> error_to_prob(mtype) |> send_to_conn(state)
    :ok
  end

  defp send_to_conn(message, %{conn_pid: pid} = _state) do
    send(pid, {:send, message})
  end

  def recv(id, message) do
    GenServer.cast(via(id), {:recv, message})
  end

  def send_message(id, message) do
    GenServer.cast(via(id), {:send, message})
  end
end

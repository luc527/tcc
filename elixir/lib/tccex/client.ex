defmodule Tccex.Client do
  use GenServer
  alias Tccex.Message
  require Logger

  defstruct sock: nil, rooms: %{}

  @type t :: %__MODULE__{sock: port, rooms: %{integer => String.t()}}

  @pingInterval 20_000

  def new, do: new([])
  def new(fields), do: struct!(__MODULE__, fields)

  def start_link(sock) do
    with {:ok, pid} = r <- GenServer.start_link(__MODULE__, sock) do
      Process.send_after(pid, :send_ping, @pingInterval)
      r
    end
  end

  @impl true
  def init(sock) when is_port(sock) do
    {:ok, new(sock: sock)}
  end

  @impl true
  def handle_info(:send_ping, %{sock: sock} = state) do
    packet = Message.encode(:ping)

    with :ok <- :gen_tcp.send(sock, packet) do
      Process.send_after(self(), :send_ping, @pingInterval)
      {:noreply, state}
    else
      {:error, reason} ->
        {:stop, exit_reason(reason), state}
    end
  end

  @impl true
  def handle_call({:recv, message}, _from, state) do
    handle_incoming(message, state)
  end

  defp handle_incoming(:pong, state), do: {:reply, :ok, state}

  # TODO: rest of them
  defp handle_incoming(msg, state) do
    {:reply, msg, state}
  end

  defp send_or_stop(message, %{sock: sock} = state) do
    packet = Message.encode(message)

    with :ok <- :gen_tcp.send(sock, packet) do
      {:reply, :ok, state}
    else
      {:error, reason} ->
        {:stop, exit_reason(reason), state}
    end
  end

  defp exit_reason(:closed), do: :normal
  defp exit_reason(reason), do: reason

  def recv(pid, message) do
    GenServer.call(pid, {:recv, message})
  end
end

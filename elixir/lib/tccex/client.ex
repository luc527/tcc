defmodule Tccex.Client do
  use GenServer
  require Tccex.Message
  alias Tccex.{Message, Client}
  require Logger

  @maxRooms 256

  defstruct sock: nil,
            rooms: %{},
            id: 0

  @type t :: %__MODULE__{
          sock: port,
          rooms: %{integer => String.t()},
          id: integer
        }

  @pingInterval 20_000

  def new, do: new([])
  def new(fields), do: struct!(__MODULE__, fields)

  def start_link({sock, id}) do
    GenServer.start_link(__MODULE__, {sock, id}, name: via(id))
  end

  defp via(id) do
    {:via, Registry, {Client.Registry, id}}
  end

  @impl true
  def init({sock, id}) when is_port(sock) and is_integer(id) do
    Process.send_after(self(), :send_ping, @pingInterval)
    Registry.register(Client.Registry, id, nil)
    {:ok, new(sock: sock, id: id)}
  end

  @impl true
  def handle_info(:send_ping, %{sock: sock} = state) do
    packet = Message.encode(:ping)

    with :ok <- :gen_tcp.send(sock, packet) do
      Process.send_after(self(), :send_ping, @pingInterval)
      {:noreply, state}
    else
      {:error, reason} ->
        {:stop, stop_reason(reason), state}
    end
  end

  @impl true
  def handle_call({:recv, message}, _from, state) do
    handle_incoming(message, state)
  end

  # mock
  defp join_room(room, name) do
    Logger.info("joining room #{room} with name #{name}")
    cond do
      name == "admin" ->
        {:error, :name_in_use}

      rem(room, 10) == 0 ->
        {:error, :room_full}

      rem(room, 11) == 0 ->
        {:error, :name_in_use}

      true ->
        :ok
    end
  end

  # mock
  defp exit_room(room) do
    Logger.info("exiting room #{room}")
    cond do
      rem(room, 10) == 0 ->
        {:error, :bad_room}

      true ->
        :ok
    end
  end

  # mock
  defp talk_in_room(room, text) do
    Logger.info("talking in room #{room}: \"#{text}\"")
    :ok
  end

  defp ok_else_error(b, e) do
    if b, do: :ok, else: {:error, e}
  end

  defp check_not_in_room(rooms, room) do
    not Map.has_key?(rooms, room) |> ok_else_error(:joined)
  end

  defp check_in_room(rooms, room) do
    Map.has_key?(rooms, room) |> ok_else_error(:bad_room)
  end

  defp check_name_is_valid(name) do
    n = byte_size(name)
    ok = n > 0 and n <= 24 and not String.contains?(name, "\n")
    ok |> ok_else_error(:bad_name)
  end

  defp check_below_room_limit(rooms) do
    (map_size(rooms) < @maxRooms) |> ok_else_error(:room_limit)
  end

  defp check_message_text(text) do
    n = byte_size(text)
    (n > 0 and n <= 2048) |> ok_else_error(:bad_message)
  end

  defp send_error_or_stop({:error, e} = error, mtype, state) do
    if Message.is_error_for(e, mtype) do
      send_or_stop({:prob, e}, state, :ok)
    else
      send_or_stop({:prob, {:transient, mtype}}, state, error)
    end
  end

  defp handle_incoming(:pong, state) do
    {:reply, :ok, state}
  end

  defp handle_incoming({:join, room, name}, %{rooms: rooms} = state) do
    with :ok <- check_not_in_room(rooms, room),
         :ok <- check_below_room_limit(rooms),
         :ok <- check_name_is_valid(name),
         :ok <- join_room(room, name) do
      state = put_in(state.rooms[room], name)
      {:reply, :ok, state}
    else
      error ->
        send_error_or_stop(error, :join, state)
    end
  end

  defp handle_incoming({:exit, room}, %{rooms: rooms} = state) do
    with :ok <- check_in_room(rooms, room),
         :ok <- exit_room(room) do
      state = update_in(state.rooms, &Map.delete(&1, room))
      {:reply, :ok, state}
    else
      error ->
        send_error_or_stop(error, :exit, state)
    end
  end

  defp handle_incoming({:talk, room, text}, %{rooms: rooms} = state) do
    with :ok <- check_in_room(rooms, room),
         :ok <- check_message_text(text),
         :ok <- talk_in_room(room, text) do
      {:reply, :ok, state}
    else
      error -> send_error_or_stop(error, :talk, state)
    end
  end

  defp handle_incoming(:lsro, %{rooms: rooms} = state) do
    send_or_stop({:rols, rooms}, state)
  end

  defp stop_reason(:closed), do: :normal
  defp stop_reason(reason), do: reason

  defp send_or_stop(message, %{sock: sock} = state, reply \\ :ok) do
    packet = Message.encode(message)

    with :ok <- :gen_tcp.send(sock, packet) do
      {:reply, reply, state}
    else
      {:error, reason} ->
        {:stop, stop_reason(reason), state}
    end
  end

  def recv(id, message) do
    GenServer.call(via(id), {:recv, message})
  end
end

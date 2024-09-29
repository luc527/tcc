defmodule Tccex.MixProject do
  use Mix.Project

  def project do
    [
      app: :tccex,
      version: "0.1.0",
      elixir: "~> 1.17",
      start_permanent: Mix.env() == :prod,
      deps: deps(),
      config_path: "config/config.exs",
    ]
  end

  def application do
    [
      extra_applications: [:logger],
      mod: {Tccex.Application, []},
    ]
  end

  defp deps do
    [
    ]
  end
end

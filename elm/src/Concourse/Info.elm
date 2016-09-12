module Concourse.Info exposing
  ( fetchVersion
  )

import Dict exposing (Dict)
import Http
import Task exposing (Task)
import String

fetchVersion : Task Http.Error String
fetchVersion =
  let
    get =
      Http.send
        Http.defaultSettings
        { verb = "GET"
        , headers = []
        , url = "/api/v1/info"
        , body = Http.empty
        }
  in
    Task.mapError promoteHttpError get `Task.andThen` handleResponse

handleResponse : Http.Response -> Task Http.Error String
handleResponse response =
  if 200 <= response.status && response.status < 300 then
    case Dict.get "x-concourse-version" <| keysToLower response.headers of
      Nothing ->
        Task.fail (Http.UnexpectedPayload "response headers should have 'X-Concourse-Version' field")

      Just version ->
        Task.succeed version
  else
    Task.fail (Http.BadResponse response.status response.statusText)

promoteHttpError : Http.RawError -> Http.Error
promoteHttpError rawError =
  case rawError of
    Http.RawTimeout -> Http.Timeout
    Http.RawNetworkError -> Http.NetworkError

keysToLower : Dict String a -> Dict String a
keysToLower = Dict.fromList << List.map fstToLower << Dict.toList

fstToLower : (String, a) -> (String, a)
fstToLower (x, y) = (String.toLower x, y)

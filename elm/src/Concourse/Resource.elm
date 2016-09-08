module Concourse.Resource exposing
  ( fetchResource
  , pause
  , unpause
  , fetchVersionedResources
  , enableVersionedResource
  , disableVersionedResource
  , fetchInputTo
  , fetchOutputOf
  )

import Concourse
import Concourse.Pagination exposing (Pagination, Paginated, Page)
import Http
import Json.Decode
import Task exposing (Task)

fetchResource : Concourse.ResourceIdentifier -> Task Http.Error Concourse.Resource
fetchResource rid =
  Http.get Concourse.decodeResource <|
    "/api/v1/teams/" ++ rid.teamName ++ "/pipelines/" ++ rid.pipelineName ++
    "/resources/" ++ rid.resourceName

pause : Concourse.ResourceIdentifier -> Task Http.Error ()
pause =
  pauseUnpause True

unpause : Concourse.ResourceIdentifier -> Task Http.Error ()
unpause =
  pauseUnpause False

pauseUnpause : Bool -> Concourse.ResourceIdentifier -> Task Http.Error ()
pauseUnpause pause rid =
  let
    action =
      if pause
        then  "pause"
        else  "unpause"
  in let
    put =
      Http.send Http.defaultSettings
        { verb = "PUT"
        , headers = []
        , url = "/api/v1/teams/" ++ rid.teamName ++ "/pipelines/" ++ rid.pipelineName ++ "/resources/" ++ rid.resourceName ++ "/" ++ action
        , body = Http.empty
        }
  in
    Task.mapError promoteHttpError put `Task.andThen` handleResponse

fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Task Http.Error (Paginated Concourse.VersionedResource)
fetchVersionedResources rid page =
  let
    url = "/api/v1/teams/" ++ rid.teamName ++ "/pipelines/" ++ rid.pipelineName ++ "/resources/" ++ rid.resourceName ++ "/versions"
  in
    Concourse.Pagination.fetch Concourse.decodeVersionedResource url page

enableVersionedResource : Concourse.VersionedResourceIdentifier -> Task Http.Error ()
enableVersionedResource =
  enableDisableVersionedResource True

disableVersionedResource : Concourse.VersionedResourceIdentifier -> Task Http.Error ()
disableVersionedResource =
  enableDisableVersionedResource False

enableDisableVersionedResource : Bool -> Concourse.VersionedResourceIdentifier -> Task Http.Error ()
enableDisableVersionedResource enable vrid =
  let
    action =
      if enable
        then  "enable"
        else  "disable"

    put =
      Http.send Http.defaultSettings
        { verb = "PUT"
        , headers = []
        , url = "/api/v1/teams/" ++ vrid.teamName ++ "/pipelines/" ++ vrid.pipelineName ++ "/resources/" ++ vrid.resourceName ++ "/versions/" ++ (toString vrid.versionID) ++ "/" ++ action
        , body = Http.empty
        }
  in
    Task.mapError promoteHttpError put `Task.andThen` handleResponse

handleResponse : Http.Response -> Task Http.Error ()
handleResponse response =
  if 200 <= response.status && response.status < 300 then
    Task.succeed ()
  else
    Task.fail (Http.BadResponse response.status response.statusText)

promoteHttpError : Http.RawError -> Http.Error
promoteHttpError rawError =
  case rawError of
    Http.RawTimeout -> Http.Timeout
    Http.RawNetworkError -> Http.NetworkError


fetchInputTo : Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Build)
fetchInputTo =
  fetchInputOutput "input_to"

fetchOutputOf : Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Build)
fetchOutputOf =
  fetchInputOutput "output_of"

fetchInputOutput : String -> Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Build)
fetchInputOutput action vrid =
  Http.get (Json.Decode.list Concourse.decodeBuild) <|
    "/api/v1/teams/" ++ vrid.teamName ++ "/pipelines/" ++ vrid.pipelineName ++ "/resources/" ++ vrid.resourceName ++ "/versions/" ++ toString vrid.versionID ++ "/" ++ action

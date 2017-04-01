module Concourse.Resource
    exposing
        ( fetchResource
        , fetchResourcesRaw
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
    Http.toTask
        << flip Http.get Concourse.decodeResource
    <|
        "/api/v1/teams/"
            ++ rid.teamName
            ++ "/pipelines/"
            ++ rid.pipelineName
            ++ "/resources/"
            ++ rid.resourceName


fetchResourcesRaw : Concourse.PipelineIdentifier -> Task Http.Error Json.Decode.Value
fetchResourcesRaw pi =
    Http.toTask <|
        flip Http.get
            Json.Decode.value
            ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/resources")


pause : Concourse.ResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pause =
    pauseUnpause True


unpause : Concourse.ResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
unpause =
    pauseUnpause False


pauseUnpause : Bool -> Concourse.ResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pauseUnpause pause rid csrfToken =
    let
        action =
            if pause then
                "pause"
            else
                "unpause"
    in
        Http.toTask <|
            Http.request
                { method = "PUT"
                , url = "/api/v1/teams/" ++ rid.teamName ++ "/pipelines/" ++ rid.pipelineName ++ "/resources/" ++ rid.resourceName ++ "/" ++ action
                , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
                , body = Http.emptyBody
                , expect = Http.expectStringResponse (\_ -> Ok ())
                , timeout = Nothing
                , withCredentials = False
                }


fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Task Http.Error (Paginated Concourse.VersionedResource)
fetchVersionedResources rid page =
    let
        url =
            "/api/v1/teams/" ++ rid.teamName ++ "/pipelines/" ++ rid.pipelineName ++ "/resources/" ++ rid.resourceName ++ "/versions"
    in
        Concourse.Pagination.fetch Concourse.decodeVersionedResource url page


enableVersionedResource : Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
enableVersionedResource =
    enableDisableVersionedResource True


disableVersionedResource : Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
disableVersionedResource =
    enableDisableVersionedResource False


enableDisableVersionedResource : Bool -> Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
enableDisableVersionedResource enable vrid csrfToken =
    let
        action =
            if enable then
                "enable"
            else
                "disable"
    in
        Http.toTask <|
            Http.request
                { method = "PUT"
                , url = "/api/v1/teams/" ++ vrid.teamName ++ "/pipelines/" ++ vrid.pipelineName ++ "/resources/" ++ vrid.resourceName ++ "/versions/" ++ (toString vrid.versionID) ++ "/" ++ action
                , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
                , body = Http.emptyBody
                , expect = Http.expectStringResponse (\_ -> Ok ())
                , timeout = Nothing
                , withCredentials = False
                }


fetchInputTo : Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Build)
fetchInputTo =
    fetchInputOutput "input_to"


fetchOutputOf : Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Build)
fetchOutputOf =
    fetchInputOutput "output_of"


fetchInputOutput : String -> Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Build)
fetchInputOutput action vrid =
    Http.toTask
        << flip Http.get (Json.Decode.list Concourse.decodeBuild)
    <|
        "/api/v1/teams/"
            ++ vrid.teamName
            ++ "/pipelines/"
            ++ vrid.pipelineName
            ++ "/resources/"
            ++ vrid.resourceName
            ++ "/versions/"
            ++ toString vrid.versionID
            ++ "/"
            ++ action

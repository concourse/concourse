module Concourse.Resource exposing
    ( check
    , enableDisableVersionedResource
    , fetchAllResources
    , fetchCausality
    , fetchInputTo
    , fetchOutputOf
    , fetchResource
    , fetchResourcesRaw
    , fetchVersionedResource
    , fetchVersionedResources
    , pinVersion
    , unpinVersion
    )

import Concourse
import Concourse.Pagination exposing (Page, Paginated, Pagination)
import Http
import Json.Decode
import Json.Encode
import Task exposing (Task)


fetchAllResources : Task Http.Error (Maybe (List Concourse.Resource))
fetchAllResources =
    Http.toTask <|
        flip Http.get
            (Json.Decode.nullable <| Json.Decode.list Concourse.decodeResource)
            "/api/v1/resources"


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


fetchVersionedResource : Concourse.VersionedResourceIdentifier -> Task Http.Error Concourse.VersionedResource
fetchVersionedResource vrid =
    Http.toTask
        << flip Http.get Concourse.decodeVersionedResource
    <|
        "/api/v1/teams/"
            ++ vrid.teamName
            ++ "/pipelines/"
            ++ vrid.pipelineName
            ++ "/resources/"
            ++ vrid.resourceName
            ++ "/versions/"
            ++ toString vrid.versionID


fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Task Http.Error (Paginated Concourse.VersionedResource)
fetchVersionedResources rid page =
    let
        url =
            "/api/v1/teams/" ++ rid.teamName ++ "/pipelines/" ++ rid.pipelineName ++ "/resources/" ++ rid.resourceName ++ "/versions"
    in
    Concourse.Pagination.fetch Concourse.decodeVersionedResource url page


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
            , url = "/api/v1/teams/" ++ vrid.teamName ++ "/pipelines/" ++ vrid.pipelineName ++ "/resources/" ++ vrid.resourceName ++ "/versions/" ++ toString vrid.versionID ++ "/" ++ action
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


fetchCausality : Concourse.VersionedResourceIdentifier -> Task Http.Error (List Concourse.Cause)
fetchCausality vrid =
    Http.toTask <|
        flip Http.get (Json.Decode.list Concourse.decodeCause) <|
            "/api/v1/teams/"
                ++ vrid.teamName
                ++ "/pipelines/"
                ++ vrid.pipelineName
                ++ "/resources/"
                ++ vrid.resourceName
                ++ "/versions/"
                ++ toString vrid.versionID
                ++ "/causality"


pinVersion : Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pinVersion vrid csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/teams/" ++ vrid.teamName ++ "/pipelines/" ++ vrid.pipelineName ++ "/resources/" ++ vrid.resourceName ++ "/versions/" ++ toString vrid.versionID ++ "/pin"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (\_ -> Ok ())
            , timeout = Nothing
            , withCredentials = False
            }


unpinVersion :
    Concourse.ResourceIdentifier
    -> Concourse.CSRFToken
    -> Task Http.Error ()
unpinVersion rid csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url =
                "/api/v1/teams/"
                    ++ rid.teamName
                    ++ "/pipelines/"
                    ++ rid.pipelineName
                    ++ "/resources/"
                    ++ rid.resourceName
                    ++ "/unpin"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (\_ -> Ok ())
            , timeout = Nothing
            , withCredentials = False
            }


check :
    Concourse.ResourceIdentifier
    -> Concourse.CSRFToken
    -> Task Http.Error ()
check rid csrfToken =
    Http.toTask <|
        Http.request
            { method = "POST"
            , url =
                "/api/v1/teams/"
                    ++ rid.teamName
                    ++ "/pipelines/"
                    ++ rid.pipelineName
                    ++ "/resources/"
                    ++ rid.resourceName
                    ++ "/check"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body =
                Http.jsonBody <|
                    Json.Encode.object [ ( "from", Json.Encode.null ) ]
            , expect = Http.expectStringResponse (\_ -> Ok ())
            , timeout = Nothing
            , withCredentials = False
            }

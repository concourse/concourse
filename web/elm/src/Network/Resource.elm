module Network.Resource exposing
    ( check
    , enableDisableVersionedResource
    , pinVersion
    , setPinComment
    , unpinVersion
    )

import Concourse
import Http
import Json.Encode
import Task exposing (Task)


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
            , url =
                "/api/v1/teams/"
                    ++ vrid.teamName
                    ++ "/pipelines/"
                    ++ vrid.pipelineName
                    ++ "/resources/"
                    ++ vrid.resourceName
                    ++ "/versions/"
                    ++ String.fromInt vrid.versionID
                    ++ "/"
                    ++ action
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (\_ -> Ok ())
            , timeout = Nothing
            , withCredentials = False
            }


pinVersion : Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pinVersion vrid csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url =
                "/api/v1/teams/"
                    ++ vrid.teamName
                    ++ "/pipelines/"
                    ++ vrid.pipelineName
                    ++ "/resources/"
                    ++ vrid.resourceName
                    ++ "/versions/"
                    ++ String.fromInt vrid.versionID
                    ++ "/pin"
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
    -> Task Http.Error Concourse.Check
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
            , expect = Http.expectJson Concourse.decodeCheck
            , timeout = Nothing
            , withCredentials = False
            }


setPinComment :
    Concourse.ResourceIdentifier
    -> Concourse.CSRFToken
    -> String
    -> Task Http.Error ()
setPinComment rid csrfToken comment =
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
                    ++ "/pin_comment"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body =
                Http.jsonBody <|
                    Json.Encode.object
                        [ ( "pin_comment"
                          , Json.Encode.string comment
                          )
                        ]
            , expect = Http.expectStringResponse (\_ -> Ok ())
            , timeout = Nothing
            , withCredentials = False
            }

module ResourceFeature exposing (all)

import Application.Application as Application
import Common exposing (and, given, then_, when)
import Data
import Dict
import Expect
import Html.Attributes as Attr
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message exposing (DomID(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Resource.Resource as Resource
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, containing, style, tag, text)


all : Test
all =
    describe "Resource page"
        [ test "unpinned versions are clickable" <|
            given iAmOnTheResourcePage
                >> and theResourceIsAlreadyPinned
                >> and iAmLookingAtAVersionOtherThanThePinnedOne
                >> when iAmLookingAtThePinButton
                >> then_ iSeeItIsClickable
        , test "clicking unpinned version sends PinResource request" <|
            given iAmOnTheResourcePage
                >> and theResourceIsAlreadyPinned
                >> when iClickTheVersionThatIsNotPinned
                >> then_ myBrowserSendsAPinResourceRequest
        , test "clicking pinned version sends UnpinResource request" <|
            given iAmOnTheResourcePage
                >> and theResourceIsAlreadyPinned
                >> when iClickTheVersionThatIsPinned
                >> then_ myBrowserSendsAnUnpinResourceRequest
        ]


iAmOnTheResourcePage _ =
    Common.init
        ("/pipelines/"
            ++ String.fromInt Data.pipelineId
            ++ "/resources/"
            ++ Data.resourceName
        )
        |> Application.handleCallback
            (Callback.AllPipelinesFetched <|
                Ok [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
            )
        |> Tuple.first


theResourceIsAlreadyPinned =
    Application.handleCallback
        (Callback.ResourceFetched <| Ok <| Data.resource (Just pinnedVersion))
        >> Tuple.first
        >> Application.handleCallback
            (Callback.VersionedResourcesFetched <|
                Ok
                    ( Resource.startingPage
                    , { content =
                            [ Data.versionedResource pinnedVersion 0
                            , Data.versionedResource notThePinnedVersion 1
                            ]
                      , pagination =
                            { previousPage = Nothing
                            , nextPage = Nothing
                            }
                      }
                    )
            )
        >> Tuple.first


pinnedVersion =
    "pinned-version"


notThePinnedVersion =
    "not-the-pinned-version"


iAmLookingAtAVersionOtherThanThePinnedOne =
    Common.queryView
        >> Query.find
            [ tag "li"
            , containing [ text notThePinnedVersion ]
            ]


iAmLookingAtThePinButton =
    Query.find
        [ attribute <|
            Attr.attribute "aria-label" "Pin Resource Version"
        ]


iSeeItIsClickable =
    Expect.all
        [ Query.has [ style "cursor" "pointer" ]
        , Event.simulate Event.click
            >> Event.expect (Update <| Message.Click <| PinButton unpinnedVersionID)
        ]


iClickTheVersionThatIsNotPinned =
    Application.update (Update <| (Message.Click <| PinButton unpinnedVersionID))


iClickTheVersionThatIsPinned =
    Application.update (Update <| (Message.Click <| PinButton pinnedVersionID))


pinnedVersionID =
    Data.resourceVersionId 0


unpinnedVersionID =
    Data.resourceVersionId 1


resourceID =
    Data.resourceId


myBrowserSendsAPinResourceRequest =
    Tuple.second >> Common.contains (Effects.DoPinVersion unpinnedVersionID)


myBrowserSendsAnUnpinResourceRequest =
    Tuple.second >> Common.contains (Effects.DoUnpinVersion resourceID)

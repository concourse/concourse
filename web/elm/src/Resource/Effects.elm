module Resource.Effects exposing (Effect(..), fetchInputAndOutputs, runEffect)

import Concourse
import Concourse.Pagination exposing (Page)
import Concourse.Resource
import Effects
import LoginRedirect
import Navigation
import Resource.Models as Models
import Resource.Msgs as Msgs
import Task
import TopBar


type Effect
    = FetchResource Concourse.ResourceIdentifier
    | FetchVersionedResources Concourse.ResourceIdentifier (Maybe Page)
    | SetTitle String
    | RedirectToLogin
    | NewUrl String
    | DoPinVersion Concourse.VersionedResourceIdentifier Concourse.CSRFToken
    | DoUnpinVersion Concourse.ResourceIdentifier Concourse.CSRFToken
    | FetchInputTo Concourse.VersionedResourceIdentifier
    | FetchOutputOf Concourse.VersionedResourceIdentifier
    | DoToggleVersion Models.VersionToggleAction Concourse.VersionedResourceIdentifier Concourse.CSRFToken
    | DoTopBarUpdate TopBar.Msg Models.Model
    | DoCheck Concourse.ResourceIdentifier Concourse.CSRFToken


runEffect : Effect -> Cmd Msgs.Msg
runEffect effect =
    case effect of
        FetchResource rid ->
            fetchResource rid

        FetchVersionedResources rid page ->
            fetchVersionedResources rid page

        SetTitle newTitle ->
            Effects.setTitle newTitle

        RedirectToLogin ->
            LoginRedirect.requestLoginRedirect ""

        NewUrl newUrl ->
            Navigation.newUrl newUrl

        DoPinVersion vrid csrfToken ->
            Task.attempt Msgs.VersionPinned <|
                Concourse.Resource.pinVersion vrid csrfToken

        DoUnpinVersion rid csrfToken ->
            Task.attempt Msgs.VersionUnpinned <|
                Concourse.Resource.unpinVersion rid csrfToken

        FetchInputTo vrid ->
            fetchInputTo vrid

        FetchOutputOf vrid ->
            fetchOutputOf vrid

        DoToggleVersion action vrid csrfToken ->
            Task.attempt (Msgs.VersionToggled action vrid.versionID) <|
                Concourse.Resource.enableDisableVersionedResource
                    (action == Models.Enable)
                    vrid
                    csrfToken

        DoTopBarUpdate msg model ->
            TopBar.update msg model
                |> Tuple.second
                |> Cmd.map Msgs.TopBarMsg

        DoCheck rid csrfToken ->
            Task.attempt Msgs.Checked <|
                Concourse.Resource.check rid csrfToken


fetchInputAndOutputs : Models.Model -> Models.Version -> List Effect
fetchInputAndOutputs model version =
    let
        identifier =
            { teamName = model.resourceIdentifier.teamName
            , pipelineName = model.resourceIdentifier.pipelineName
            , resourceName = model.resourceIdentifier.resourceName
            , versionID = version.id
            }
    in
    [ FetchInputTo identifier
    , FetchOutputOf identifier
    ]


fetchResource : Concourse.ResourceIdentifier -> Cmd Msgs.Msg
fetchResource resourceIdentifier =
    Task.attempt Msgs.ResourceFetched <|
        Concourse.Resource.fetchResource resourceIdentifier


fetchVersionedResources :
    Concourse.ResourceIdentifier
    -> Maybe Page
    -> Cmd Msgs.Msg
fetchVersionedResources resourceIdentifier page =
    Task.attempt (Msgs.VersionedResourcesFetched page) <|
        Concourse.Resource.fetchVersionedResources resourceIdentifier page


fetchInputTo : Concourse.VersionedResourceIdentifier -> Cmd Msgs.Msg
fetchInputTo versionedResourceIdentifier =
    Task.attempt (Msgs.InputToFetched versionedResourceIdentifier.versionID) <|
        Concourse.Resource.fetchInputTo versionedResourceIdentifier


fetchOutputOf : Concourse.VersionedResourceIdentifier -> Cmd Msgs.Msg
fetchOutputOf versionedResourceIdentifier =
    Task.attempt (Msgs.OutputOfFetched versionedResourceIdentifier.versionID) <|
        Concourse.Resource.fetchOutputOf versionedResourceIdentifier

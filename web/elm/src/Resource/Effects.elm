module Resource.Effects exposing (Effect(..), fetchInputAndOutputs, runEffect)

import Concourse
import Concourse.Pagination exposing (Page)
import Concourse.Resource
import Effects exposing (setTitle)
import LoginRedirect
import Navigation
import Resource.Models as Models
import Resource.Msgs exposing (Msg(..))
import Task
import TopBar


type Effect
    = FetchResource Concourse.ResourceIdentifier
    | FetchVersionedResources Concourse.ResourceIdentifier (Maybe Page)
    | SetTitle String
    | RedirectToLogin
    | NavigateTo String
    | DoPinVersion Concourse.VersionedResourceIdentifier Concourse.CSRFToken
    | DoUnpinVersion Concourse.ResourceIdentifier Concourse.CSRFToken
    | FetchInputTo Concourse.VersionedResourceIdentifier
    | FetchOutputOf Concourse.VersionedResourceIdentifier
    | DoToggleVersion Models.VersionToggleAction Concourse.VersionedResourceIdentifier Concourse.CSRFToken
    | DoTopBarUpdate TopBar.Msg Models.Model
    | DoCheck Concourse.ResourceIdentifier Concourse.CSRFToken


runEffect : Effect -> Cmd Msg
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

        NavigateTo newUrl ->
            Navigation.newUrl newUrl

        DoPinVersion version csrfToken ->
            Task.attempt VersionPinned <|
                Concourse.Resource.pinVersion version csrfToken

        DoUnpinVersion id csrfToken ->
            Task.attempt VersionUnpinned <|
                Concourse.Resource.unpinVersion id csrfToken

        FetchInputTo id ->
            fetchInputTo id

        FetchOutputOf id ->
            fetchOutputOf id

        DoToggleVersion action vrid csrfToken ->
            Task.attempt (VersionToggled action vrid.versionID) <|
                Concourse.Resource.enableDisableVersionedResource
                    (action == Models.Enable)
                    vrid
                    csrfToken

        DoTopBarUpdate msg model ->
            TopBar.update msg model
                |> Tuple.second
                |> Cmd.map TopBarMsg

        DoCheck rid csrfToken ->
            Task.attempt Checked <|
                Concourse.Resource.check rid csrfToken


fetchResource : Concourse.ResourceIdentifier -> Cmd Msg
fetchResource resourceIdentifier =
    Task.attempt ResourceFetched <|
        Concourse.Resource.fetchResource resourceIdentifier


fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Cmd Msg
fetchVersionedResources resourceIdentifier page =
    Task.attempt (VersionedResourcesFetched page) <|
        Concourse.Resource.fetchVersionedResources resourceIdentifier page


fetchInputTo : Concourse.VersionedResourceIdentifier -> Cmd Msg
fetchInputTo versionedResourceIdentifier =
    Task.attempt (InputToFetched versionedResourceIdentifier.versionID) <|
        Concourse.Resource.fetchInputTo versionedResourceIdentifier


fetchOutputOf : Concourse.VersionedResourceIdentifier -> Cmd Msg
fetchOutputOf versionedResourceIdentifier =
    Task.attempt (OutputOfFetched versionedResourceIdentifier.versionID) <|
        Concourse.Resource.fetchOutputOf versionedResourceIdentifier


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

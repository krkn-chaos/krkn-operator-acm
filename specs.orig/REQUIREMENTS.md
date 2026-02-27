# Overview 
this is an operator used to integrate with RedHat ACM (Advanced Cluster Management) and a custom made operator
called krkn-operator.

# Requirements

- the operator must react to the creation of a CRD called `KrknTargetRequest`, this CRD contains a UUID, a targetData field
  (that will be a json that I'll describe later) and a status field
- the `KrknTargetRequest` CR is created by another operator with a valorized UUID and status to `pending`

once the operator receives the `KrknTargetRequest` the operator must:

- make an api request to  /apis/cluster.open-cluster-management.io/v1/managedclusters, the response payload can be found
  in misc/managed-cluster.json. This Api request goal is to list and collect the available 
  - cluster names (jsonpath .items[].name)
  - cluster API URL (jsonpath .items[].spec.managedClusterClientConfigs[].url)
  - cluster CA Bundle (jsonpath .items[].spec.managedClusterClientConfigs[].caBunble)
  - Each cluster name will have a namespace with the same name of the cluster containing a secret called `application-manager`
    - `application-manager` secret has two fields base64 encoded
      - ca.crt (containing the ca certificate of the cluster api)
      - token (the token that is used to query the managed cluster API)
- with all the information gathered must build a valid kubeconfig per each cluster
- create a secret which name is the UUID of the request
  - the secret must have a field .data.managed-clusters
    - the data must be a json map key with the name of the cluster and value an object 
    - the object must contain the cluster-name, cluster-api and kubeconfig (base64 encoded)
- when the secret is created the `KrknTargetRequest` status must be changed to Completed and the field targetData will contain a list of:
  - cluster-name
  - cluster-api-url

# Refactoring 28/11

- I want to add a New CRD called KrknOperatorTargetProvider, the goal is to register the operator with its operator-name and a timestamp
  At the operator boot, if the CR already exists with the same operator-name only the timestamp must be updated
- The operator name must come from a config-map this, configmap will progressively contain many other options so must be reachable
  from almost any place in the reconcile loop 
- I want to add two timestamp to KrknTargetRequest one is `created` the other one is `completed` and they must be valorized
  respectively when the CR is created and one when it's completed
- I want to change both the CRD KrknTargetRequestStatus , the attribute TargetData []ClusterTarget must become a map[str][]ClusterTarget
  where the key is the operator-name and the value are the targets associated with the operator itself, the aim is to allow multiple
  operator with different data sources might set values on the same CR
- I want to make the same change also in the json data structure in the Secret
- on every reconcile loop the operator must list the number of 
- 
- I want that in the reconcile loop the status of the CR must be completed when the number of keys in the map[str][]ClusterTarget
  
- equals the number of KrknOperatorTargetProvider listed before

# Refactoring 2 28/11

- I want to add a field active bool al CRD KrknOperatorTargetProvider
- I want the reconcile loop to count only the target provider that are Active == true
- I want to add the operator namespace in the global configmap
- I want to create a config object that is mapped in a methdo from the values contained in the configmap and returned from
  that method, and I want to remove the single property collection methods like `getOperatorName`
- check  the krkntargetrequest_controller.go line 133 `Requeue` is deprecated, find a solution.

# KrknOperatorTargetProviderConfig reconciliation and ConfigMap setting

I want to have a controller for KrknOperatorTargetProviderConfig CRD from https://github.com/krkn-chaos/krkn-operator/api/v1alpha1.
In the reconcile loop the CR must be updated using the https://github.com/krkn-chaos/krkn-operator/pkg/provider/config.go methods.

## config schema 

the config schema must be built on top of krknctl typing system exposed by https://github.com/krkn-chaos/krknctl/pkg/typing.
The first target is to build a list of the available secret for each cluster namespace:
- list the managedcluster CRs
- for each cluster in the list a namespace is available
- for each cluster a field must be created:
- - the field must be of type enum
- - the separator must be a ","
- - the comma separated values are the secrets available in the namespace sorted
- - the default value is "application-manager" secret if present in the list otherwise the first available
- - the variable must be ACM_SECRET_<NAMESPACE>
- - - namespace must be capitalized an - replaced by _
- - - there must be a method to format the namespace
-  the field list must be serialized to json and passed to the UpdateProviderConfig as jsonSchema parameter
- operatorName will come from the same const used to register the KrknOperatorTargetProvider puoi

## ConfigMap controller
I want a controller that reconciles the configuration config map which name is defined in the constraints DefaultConfigMapName.
Every change in the config map must be reflected in the configstore singleton. The singleton must be instantiated in the main.go.

## krkntargetrequest adaptation
when the kubeconfig is generated instead of selecting "application-manager" as default secret I want that the configstore is queried 
for the selected target cluster (normalizing the name before making the query) and use the provided secret.




 


  

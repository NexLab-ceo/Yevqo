<script>export let clickable=false;export let padding='md';</script>
<div class="card card-{padding}" class:clickable on:click on:keydown role={clickable?'button':undefined} tabindex={clickable?0:undefined}><slot /></div>
<style>
.card{background:#101727;border:1px solid #253149;border-radius:14px;transition:border-color .15s}.card.clickable{cursor:pointer}.card.clickable:hover{border-color:#3b4b68}.card.clickable:focus{outline:2px solid #818cf8;outline-offset:2px}.card-sm{padding:12px}.card-md{padding:17px}.card-lg{padding:24px}
</style>